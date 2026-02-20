package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/dvrd/hound/internal/blockchain"
	"github.com/dvrd/hound/internal/config"
	"github.com/dvrd/hound/internal/database"
	"github.com/dvrd/hound/internal/dex"
	"github.com/dvrd/hound/internal/models"
	"github.com/dvrd/hound/internal/services"
	"github.com/dvrd/hound/internal/swap"
	"github.com/dvrd/hound/internal/tui"
	"github.com/dvrd/hound/internal/tui/views/history"
	"github.com/dvrd/hound/internal/tui/views/swapview"
	"github.com/dvrd/hound/internal/tui/views/tokenadd"
	"github.com/dvrd/hound/internal/tui/views/tokenfetch"
	"github.com/dvrd/hound/internal/tui/views/tokenlist"
	"github.com/dvrd/hound/internal/tui/views/walletdelete"
	"github.com/dvrd/hound/internal/tui/views/walletimport"
	"github.com/dvrd/hound/internal/tui/views/walletlist"
	"github.com/dvrd/hound/internal/tui/views/walletstatus"
	"github.com/dvrd/hound/internal/wallet"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

var jsonOutput bool

func main() {
	rootCmd := &cobra.Command{
		Use:   "hound",
		Short: "Solana wallet management TUI",
		Long:  "Hound — Solana wallet management, portfolio tracking, and token swaps",
		RunE:  runTUI, // Default: launch TUI
	}

	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON (non-interactive)")

	// Subcommands
	rootCmd.AddCommand(walletCmd())
	rootCmd.AddCommand(tokensCmd())
	rootCmd.AddCommand(historyCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// deps holds all initialized dependencies for the application.
type deps struct {
	db           *database.Database
	walletMgr    *wallet.WalletManager
	keystoreSvc  *services.KeystoreService
	cfg          config.Config
	rpcClient    *blockchain.RPCClient
	dexscreener  *dex.DexScreenerClient
	jupiter      *dex.JupiterClient
	swapClient   *swap.SwapClient
	swapSvc      *services.SwapService
	tokenInfoSvc *services.TokenInfoService
}

func initDeps() (*deps, error) {
	cfg := config.DefaultConfig()
	if err := config.EnsureConfigDir(); err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	if err := db.MigrateSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration: %w", err)
	}
	if err := db.CreateSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}

	rpcClient := blockchain.NewRPCClient(cfg.RPCEndpoint, cfg.BackupEndpoints)
	jupiterClient := dex.NewJupiterClient()
	dexscreenerClient := dex.NewDexScreenerClient()
	router := dex.NewRouter(rpcClient, jupiterClient)
	balanceFetcher := wallet.NewBalanceFetcher(rpcClient, router, db)
	walletMgr := wallet.NewWalletManager(db, balanceFetcher)
	keystoreSvc := &services.KeystoreService{}
	swapClient := swap.NewSwapClient()
	swapSvc := services.NewSwapService(swapClient, keystoreSvc, db)
	tokenInfoSvc := services.NewTokenInfoService(dexscreenerClient, rpcClient)

	return &deps{
		db:           db,
		walletMgr:    walletMgr,
		keystoreSvc:  keystoreSvc,
		cfg:          cfg,
		rpcClient:    rpcClient,
		dexscreener:  dexscreenerClient,
		jupiter:      jupiterClient,
		swapClient:   swapClient,
		swapSvc:      swapSvc,
		tokenInfoSvc: tokenInfoSvc,
	}, nil
}

// makeViewFactory creates a ViewFactory that maps view names to concrete view models.
func makeViewFactory(d *deps) tui.ViewFactory {
	return func(name string, data interface{}) tea.Model {
		switch name {
		case "wallet-list":
			m := walletlist.New(d.walletMgr, d.db)
			return m

		case "wallet-import":
			m := walletimport.New(d.db, d.keystoreSvc)
			return m

		case "wallet-status":
			addr, _ := data.(string)
			m := walletstatus.New(d.walletMgr, addr)
			return m

		case "wallet-delete":
			w, ok := data.(models.Wallet)
			if !ok {
				return nil
			}
			// Get wallet count for the "can't delete last wallet" check
			wallets, err := d.db.GetAllWallets()
			count := 0
			if err == nil {
				count = len(wallets)
			}
			m := walletdelete.New(w, d.db, count)
			return m

		case "token-list":
			m := tokenlist.New(d.db)
			return m

		case "token-fetch":
			mintOrSymbol, _ := data.(string)
			m := tokenfetch.New(mintOrSymbol, d.tokenInfoSvc, d.db)
			return m

		case "token-add":
			m := tokenadd.New(d.db)
			return m

		case "swap":
			// Data can be a wallet address string
			addr, _ := data.(string)
			if addr == "" {
				// Use primary wallet
				if pw, err := d.walletMgr.GetPrimaryWallet(); err == nil {
					addr = pw.Address
				}
			}
			m := swapview.New(addr, d.swapClient, d.swapSvc, false)
			return m

		case "history":
			addr, _ := data.(string)
			m := history.New(addr, d.db)
			return m

		default:
			return nil
		}
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	d, err := initDeps()
	if err != nil {
		return err
	}
	defer d.db.Close()

	factory := makeViewFactory(d)
	app := tui.NewApp(d.db, d.walletMgr, d.keystoreSvc, d.cfg, factory)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

func walletCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "Wallet management commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all wallets",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return runWalletListJSON()
			}
			return runTUI(cmd, args)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status [address|label]",
		Short: "Show wallet portfolio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				identifier := ""
				if len(args) > 0 {
					identifier = args[0]
				}
				return runWalletStatusJSON(identifier)
			}
			return runTUI(cmd, args)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "import",
		Short: "Import a wallet from seed phrase",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Always interactive — launch TUI at import view
			return runTUI(cmd, args)
		},
	})

	return cmd
}

func tokensCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Token management commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List tracked tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return runTokensListJSON()
			}
			return runTUI(cmd, args)
		},
	})

	return cmd
}

func historyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "View swap history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return runHistoryJSON()
			}
			return runTUI(cmd, args)
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("hound " + version)
		},
	}
}

// runWalletListJSON outputs all wallets as JSON.
func runWalletListJSON() error {
	d, err := initDeps()
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}
	defer d.db.Close()

	wallets, err := d.walletMgr.GetWallets()
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}

	result := make([]tui.WalletJSON, 0, len(wallets))
	for _, w := range wallets {
		p, _ := d.walletMgr.GetCachedPortfolio(w.Address)
		result = append(result, tui.WalletToJSON(w, p.TotalUSD))
	}

	return tui.PrintJSON(result)
}

// runWalletStatusJSON outputs a wallet's portfolio as JSON.
func runWalletStatusJSON(identifier string) error {
	d, err := initDeps()
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}
	defer d.db.Close()

	w, err := d.walletMgr.ResolveWallet(identifier)
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}

	portfolio, err := d.walletMgr.RefreshPortfolio(w.Address)
	if err != nil {
		// Fall back to cached
		portfolio, err = d.walletMgr.GetCachedPortfolio(w.Address)
		if err != nil {
			tui.PrintError(err)
			os.Exit(1)
		}
	}

	return tui.PrintJSON(tui.PortfolioToJSON(portfolio))
}

// runTokensListJSON outputs all tracked tokens as JSON.
func runTokensListJSON() error {
	d, err := initDeps()
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}
	defer d.db.Close()

	tokens, err := d.db.GetAllTokens()
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}

	result := make([]tui.TokenListJSON, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, tui.TokenToJSON(t, len(t.Pools)))
	}

	return tui.PrintJSON(result)
}

// runHistoryJSON outputs swap history as JSON.
func runHistoryJSON() error {
	d, err := initDeps()
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}
	defer d.db.Close()

	entries, err := d.db.GetSwapHistory("", 50)
	if err != nil {
		tui.PrintError(err)
		os.Exit(1)
	}

	result := make([]tui.SwapHistoryJSON, 0, len(entries))
	for _, e := range entries {
		result = append(result, tui.SwapHistoryToJSON(e))
	}

	return tui.PrintJSON(result)
}
