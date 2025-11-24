# Configurar Helius RPC para Hound

Esta guía te ayuda a configurar Helius (un RPC rápido y gratuito) para mejorar significativamente el rendimiento de Hound.

## ¿Por qué Helius?

- **Gratis**: 100,000 requests/día en el tier gratuito
- **Rápido**: ~100ms de latencia (vs ~2 segundos del RPC público)
- **Confiable**: 99.9% uptime garantizado
- **Sin configuración compleja**: Solo necesitas una API key

## Paso 1: Crear cuenta en Helius

1. Ve a: https://dashboard.helius.dev/signup
2. Sign up con:
   - GitHub (recomendado)
   - Google
   - Email

3. Confirma tu email si usaste email directo

## Paso 2: Crear API Key

1. Una vez logueado, ve a: https://dashboard.helius.dev
2. Click en **"Create new API Key"**
3. Nombre: `hound-local` (o el que prefieras)
4. Network: **Mainnet**
5. Click **"Create"**

6. **Copia tu API key** - se verá algo así:
   ```
   a1b2c3d4-e5f6-7890-abcd-ef1234567890
   ```

## Paso 3: Configurar Hound

### Opción A: Variable de entorno (Recomendado)

Agrega esto a tu `~/.zshrc` o `~/.bashrc`:

```bash
export HOUND_RPC_ENDPOINT="https://mainnet.helius-rpc.com/?api-key=TU_API_KEY_AQUI"
```

Luego recarga tu shell:
```bash
source ~/.zshrc  # o ~/.bashrc
```

### Opción B: Archivo .env (Para desarrollo)

1. Copia el archivo de ejemplo:
   ```bash
   cp .env.example .env
   ```

2. Edita `.env` y reemplaza con tu API key:
   ```bash
   HOUND_RPC_ENDPOINT=https://mainnet.helius-rpc.com/?api-key=TU_API_KEY_AQUI
   ```

3. Carga las variables antes de ejecutar hound:
   ```bash
   source .env
   ./bin/hound wallet status
   ```

## Paso 4: Verificar

Prueba que funciona:

```bash
# Con logs de debug para ver el endpoint
./bin/hound_debug wallet status
```

Deberías ver:
```
[INFO] Using custom RPC endpoint: https://mainnet.helius-rpc.com/?api-key=...
```

Y el comando debería completarse en **< 2 segundos** (vs timeout con el RPC público).

## Troubleshooting

### Error: "Rate limit exceeded"
- Has excedido 100K requests/día
- Solución: Espera hasta mañana o upgrade a plan pagado

### Error: "Invalid API key"
- Verifica que copiaste la API key completa
- Verifica que no haya espacios extra
- Verifica que la URL esté bien formada

### Sigue lento
- Verifica que la variable de entorno esté configurada:
  ```bash
  echo $HOUND_RPC_ENDPOINT
  ```
- Debería mostrar tu URL de Helius, no el RPC público

## Alternativas

Si Helius no funciona, también puedes usar:

### QuickNode
```bash
export HOUND_RPC_ENDPOINT="https://your-endpoint.quiknode.pro/YOUR_KEY/"
```
- Tier gratuito: 50K requests/día
- Sign up: https://www.quicknode.com

### Alchemy
```bash
export HOUND_RPC_ENDPOINT="https://solana-mainnet.g.alchemy.com/v2/YOUR_API_KEY"
```
- Tier gratuito: 300M compute units/mes
- Sign up: https://www.alchemy.com

### Ankr (No requiere API key)
```bash
export HOUND_RPC_ENDPOINT="https://rpc.ankr.com/solana"
```
- Tier gratuito: 500 req/s (rate limited)
- No sign up requerido

## Monitorear uso

Helius dashboard muestra:
- Requests totales hoy
- Requests por hora
- Rate limit status
- Uptime

Ve a: https://dashboard.helius.dev

## Límites del tier gratuito

- **100,000 requests/día**
- **30 requests/segundo**
- **5 concurrent connections**

Para Hound (uso personal), esto es más que suficiente.

## Upgrade a plan pagado (opcional)

Si necesitas más:
- **Developer**: $50/mes - 1M requests/día
- **Pro**: $250/mes - 10M requests/día
- **Enterprise**: Custom pricing

Pero para Hound personal, el tier gratuito debería ser suficiente.
