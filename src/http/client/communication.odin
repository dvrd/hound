#+private
package client

import "core:bufio"
import "core:bytes"
import "core:c"
import "core:io"
import "core:log"
import "core:net"
import "core:strconv"
import "core:strings"
import "core:sys/posix"

import http ".."
import openssl "../openssl"

// resolve_hostname_native uses native OS DNS resolution on macOS via getaddrinfo()
// This is necessary because core:net reads /etc/resolv.conf which is not used on macOS.
// macOS uses mDNSResponder accessed via getaddrinfo() system call.
when ODIN_OS == .Darwin {
	resolve_hostname_native :: proc(hostname: string) -> (ep4, ep6: net.Endpoint, err: net.Network_Error) {
		// Convert hostname to C string
		hostname_cstr := strings.clone_to_cstring(hostname, context.temp_allocator)

		// Setup hints for getaddrinfo
		hints: posix.addrinfo = {}
		hints.ai_family = .UNSPEC     // Allow IPv4 or IPv6
		hints.ai_socktype = .STREAM   // TCP socket
		hints.ai_flags = {.ADDRCONFIG}  // Only return addresses if we have that type configured

		// Call getaddrinfo
		result: ^posix.addrinfo = nil
		ret := posix.getaddrinfo(hostname_cstr, nil, &hints, &result)
		if ret != .NONE {
			// Failed to resolve
			err = .Unable_To_Resolve
			return
		}
		defer posix.freeaddrinfo(result)

		// Parse results - iterate through linked list
		for res := result; res != nil; res = res.ai_next {
			if res.ai_family == .INET && ep4.address == nil {
				// IPv4 address - extract bytes from u32be
				sockaddr_in := (^posix.sockaddr_in)(res.ai_addr)
				addr_u32 := sockaddr_in.sin_addr.s_addr

				// Convert u32be to [4]u8 in network byte order
				addr_bytes := transmute([4]u8)addr_u32
				ep4.address = net.IP4_Address{
					addr_bytes[0],
					addr_bytes[1],
					addr_bytes[2],
					addr_bytes[3],
				}
			} else if res.ai_family == .INET6 && ep6.address == nil {
				// IPv6 address - s6_addr is [16]u8, convert to [8]u16be
				sockaddr_in6 := (^posix.sockaddr_in6)(res.ai_addr)
				addr_u8 := sockaddr_in6.sin6_addr.s6_addr

				// Convert [16]u8 to [8]u16be (network byte order)
				ep6.address = net.IP6_Address{
					u16be((u16(addr_u8[0]) << 8) | u16(addr_u8[1])),
					u16be((u16(addr_u8[2]) << 8) | u16(addr_u8[3])),
					u16be((u16(addr_u8[4]) << 8) | u16(addr_u8[5])),
					u16be((u16(addr_u8[6]) << 8) | u16(addr_u8[7])),
					u16be((u16(addr_u8[8]) << 8) | u16(addr_u8[9])),
					u16be((u16(addr_u8[10]) << 8) | u16(addr_u8[11])),
					u16be((u16(addr_u8[12]) << 8) | u16(addr_u8[13])),
					u16be((u16(addr_u8[14]) << 8) | u16(addr_u8[15])),
				}
			}

			// Stop if we have both IPv4 and IPv6
			if ep4.address != nil && ep6.address != nil {
				break
			}
		}

		// Return error if we couldn't resolve anything
		if ep4.address == nil && ep6.address == nil {
			err = .Unable_To_Resolve
			return
		}

		return
	}
}

parse_endpoint :: proc(target: string) -> (url: http.URL, endpoint: net.Endpoint, err: net.Network_Error) {
	url = http.url_parse(target)
	host_or_endpoint := net.parse_hostname_or_endpoint(url.host) or_return

	switch t in host_or_endpoint {
	case net.Endpoint:
		endpoint = t
		return
	case net.Host:
		// Use native DNS resolution on macOS to work around /etc/resolv.conf issue
		when ODIN_OS == .Darwin {
			ep4, ep6 := resolve_hostname_native(t.hostname) or_return
			endpoint = ep4 if ep4.address != nil else ep6
		} else {
			ep4, ep6 := net.resolve(t.hostname) or_return
			endpoint = ep4 if ep4.address != nil else ep6
		}

		endpoint.port = t.port
		if endpoint.port == 0 {
			endpoint.port = url.scheme == "https" ? 443 : 80
		}
		return
	case:
		unreachable()
	}
}

format_request :: proc(target: http.URL, request: ^Request, allocator := context.allocator) -> (buf: bytes.Buffer) {
	// Responses are on average at least 100 bytes, so lets start there, but add the body's length.
	bytes.buffer_init_allocator(&buf, 0, bytes.buffer_length(&request.body) + 100, allocator)

	http.requestline_write(
		bytes.buffer_to_stream(&buf),
		{method = request.method, target = target, version = http.Version{1, 1}},
	)

	if !http.headers_has_unsafe(request.headers, "content-length") {
		buf_len := bytes.buffer_length(&request.body)
		if buf_len == 0 {
			bytes.buffer_write_string(&buf, "content-length: 0\r\n")
		} else {
			bytes.buffer_write_string(&buf, "content-length: ")

			// Make sure at least 20 bytes are there to write into, should be enough for the content length.
			bytes.buffer_grow(&buf, buf_len + 20)

			// Write the length into unwritten portion.
			unwritten := http._dynamic_unwritten(buf.buf)
			l := len(strconv.write_int(unwritten, i64(buf_len), 10))
			assert(l <= 20)
			http._dynamic_add_len(&buf.buf, l)

			bytes.buffer_write_string(&buf, "\r\n")
		}
	}

	if !http.headers_has_unsafe(request.headers, "accept") {
		bytes.buffer_write_string(&buf, "accept: */*\r\n")
	}

	if !http.headers_has_unsafe(request.headers, "user-agent") {
		bytes.buffer_write_string(&buf, "user-agent: odin-http\r\n")
	}

	if !http.headers_has_unsafe(request.headers, "host") {
		bytes.buffer_write_string(&buf, "host: ")
		bytes.buffer_write_string(&buf, target.host)
		bytes.buffer_write_string(&buf, "\r\n")
	}

	for header, value in request.headers._kv {
		bytes.buffer_write_string(&buf, header)
		bytes.buffer_write_string(&buf, ": ")

		// Escape newlines in headers, if we don't, an attacker can find an endpoint
		// that returns a header with user input, and inject headers into the response.
		esc_value, was_allocation := strings.replace_all(value, "\n", "\\n", allocator)
		defer if was_allocation { delete(esc_value) }

		bytes.buffer_write_string(&buf, esc_value)
		bytes.buffer_write_string(&buf, "\r\n")
	}

	if len(request.cookies) > 0 {
		bytes.buffer_write_string(&buf, "cookie: ")

		for cookie, i in request.cookies {
			bytes.buffer_write_string(&buf, cookie.name)
			bytes.buffer_write_byte(&buf, '=')
			bytes.buffer_write_string(&buf, cookie.value)

			if i != len(request.cookies) - 1 {
				bytes.buffer_write_string(&buf, "; ")
			}
		}

		bytes.buffer_write_string(&buf, "\r\n")
	}

	// Empty line denotes end of headers and start of body.
	bytes.buffer_write_string(&buf, "\r\n")

	bytes.buffer_write(&buf, bytes.buffer_to_bytes(&request.body))
	return
}

SSL_Communication :: struct {
	socket: net.TCP_Socket,
	ssl:    ^openssl.SSL,
	ctx:    ^openssl.SSL_CTX,
}

Communication :: union {
	net.TCP_Socket, // HTTP.
	SSL_Communication, // HTTPS.
}

parse_response :: proc(socket: Communication, allocator := context.allocator) -> (res: Response, err: Error) {
	res._socket = socket

	stream: io.Stream
	switch comm in socket {
	case net.TCP_Socket:
		stream = tcp_stream(comm)
	case SSL_Communication:
		stream = ssl_tcp_stream(comm.ssl)
	}

	stream_reader := io.to_reader(stream)
	scanner: bufio.Scanner
	bufio.scanner_init(&scanner, stream_reader, allocator)

	http.headers_init(&res.headers, allocator)

	if !bufio.scanner_scan(&scanner) {
		err = bufio.scanner_error(&scanner)
		return
	}

	rline_str := bufio.scanner_text(&scanner)
	si := strings.index_byte(rline_str, ' ')

	version, ok := http.version_parse(rline_str[:si])
	if !ok {
		err = Request_Error.Invalid_Response_HTTP_Version
		return
	}

	// Might need to support more versions later.
	if version.major != 1 {
		err = Request_Error.Invalid_Response_HTTP_Version
		return
	}

	res.status, ok = http.status_from_string(rline_str[si + 1:])
	if !ok {
		err = Request_Error.Invalid_Response_Method
		return
	}

	res.cookies.allocator = allocator

	for {
		if !bufio.scanner_scan(&scanner) {
			err = bufio.scanner_error(&scanner)
			return
		}

		line := bufio.scanner_text(&scanner)
		// Empty line means end of headers.
		if line == "" { break }

		key, hok := http.header_parse(&res.headers, line, allocator)
		if !hok {
			err = Request_Error.Invalid_Response_Header
			return
		}

		if key == "set-cookie" {
			cookie_str := http.headers_get_unsafe(res.headers, "set-cookie")
			http.headers_delete_unsafe(&res.headers, "set-cookie")
			delete(key, allocator)

			cookie, cok := http.cookie_parse(cookie_str, allocator)
			if !cok {
				err = Request_Error.Invalid_Response_Cookie
				return
			}

			append(&res.cookies, cookie)
		}
	}

	if !http.headers_validate(&res.headers) {
		err = Request_Error.Invalid_Response_Header
		return
	}

	res.headers.readonly = true

	res._body = scanner
	return res, nil
}

ssl_tcp_stream :: proc(sock: ^openssl.SSL) -> (s: io.Stream) {
	s.data = sock
	s.procedure = _ssl_stream_proc
	return s
}

@(private)
_ssl_stream_proc :: proc(
	stream_data: rawptr,
	mode: io.Stream_Mode,
	p: []byte,
	offset: i64,
	whence: io.Seek_From,
) -> (
	n: i64,
	err: io.Error,
) {
	#partial switch mode {
	case .Query:
		return io.query_utility(io.Stream_Mode_Set{.Query, .Read})
	case .Read:
		ssl := cast(^openssl.SSL)stream_data
		ret := openssl.SSL_read(ssl, raw_data(p), c.int(len(p)))
		if ret <= 0 {
			return 0, .Unexpected_EOF
		}

		return i64(ret), nil
	case:
		err = .Empty
	}
	return
}

// Wraps a tcp socket with a stream.
tcp_stream :: proc(sock: net.TCP_Socket) -> (s: io.Stream) {
	s.data = rawptr(uintptr(sock))
	s.procedure = _socket_stream_proc
	return s
}

@(private)
_socket_stream_proc :: proc(
	stream_data: rawptr,
	mode: io.Stream_Mode,
	p: []byte,
	offset: i64,
	whence: io.Seek_From,
) -> (
	n: i64,
	err: io.Error,
) {
	#partial switch mode {
	case .Query:
		return io.query_utility(io.Stream_Mode_Set{.Query, .Read})
	case .Read:
		sock := net.TCP_Socket(uintptr(stream_data))
		received, recv_err := net.recv_tcp(sock, p)
		n = i64(received)

		#partial switch recv_err {
		case .None:
			err = .None
		case .Network_Unreachable, .Insufficient_Resources, .Invalid_Argument, .Not_Connected, .Connection_Closed, .Timeout, .Would_Block, .Interrupted:
			log.errorf("unexpected error reading tcp: %s", recv_err)
			err = .Unexpected_EOF
		case:
			log.errorf("unexpected error reading tcp: %s", recv_err)
			err = .Unknown
		}
		case nil:
			err = .None
		case:
			assert(false, "recv_tcp only returns TCP_Recv_Error or nil")
		}
	return
}
