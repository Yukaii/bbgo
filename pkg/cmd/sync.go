package cmd

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/c9s/bbgo/pkg/bbgo"
)

func init() {
	SyncCmd.Flags().String("session", "", "the exchange session name for sync")
	SyncCmd.Flags().String("symbol", "", "symbol of market for syncing")
	SyncCmd.Flags().String("since", "", "sync from time")
	RootCmd.AddCommand(SyncCmd)
}

var SyncCmd = &cobra.Command{
	Use:          "sync [--session=[exchange_name]] [--symbol=[pair_name]] [[--since=yyyy/mm/dd]]",
	Short:        "sync trades and orders history",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		if len(configFile) == 0 {
			return errors.New("--config option is required")
		}

		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return err
		}

		userConfig, err := bbgo.Load(configFile, false)
		if err != nil {
			return err
		}

		since, err := cmd.Flags().GetString("since")
		if err != nil {
			return err
		}

		environ := bbgo.NewEnvironment()
		if err := environ.ConfigureDatabase(ctx); err != nil {
			return err
		}

		if err := environ.ConfigureExchangeSessions(userConfig); err != nil {
			return err
		}

		sessionName, err := cmd.Flags().GetString("session")
		if err != nil {
			return err
		}

		symbol, err := cmd.Flags().GetString("symbol")
		if err != nil {
			return err
		}

		var (
			// default sync start time
			defaultSyncStartTime = time.Now().AddDate(-1, 0, 0)
		)

		var syncStartTime = defaultSyncStartTime

		if userConfig.Sync != nil && userConfig.Sync.Since != nil {
			syncStartTime = userConfig.Sync.Since.Time()
		}

		if len(since) > 0 {
			syncStartTime, err = time.ParseInLocation("2006-01-02", since, time.Local)
			if err != nil {
				return err
			}
		}

		environ.SetSyncStartTime(syncStartTime)

		// syncSymbols is the symbol list to sync
		var syncSymbols []string

		if userConfig.Sync != nil && len(userConfig.Sync.Symbols) > 0 {
			syncSymbols = userConfig.Sync.Symbols
		}

		if len(symbol) > 0 {
			syncSymbols = []string{symbol}
		}

		var selectedSessions []string

		if userConfig.Sync != nil && len(userConfig.Sync.Sessions) > 0 {
			selectedSessions = userConfig.Sync.Sessions
		}
		if len(sessionName) > 0 {
			selectedSessions = []string{sessionName}
		}

		sessions := environ.SelectSessions(selectedSessions...)
		for _, session := range sessions {
			if err := environ.SyncSession(ctx, session, syncSymbols...); err != nil {
				return err
			}

			log.Infof("exchange session %s synchronization done", session.Name)
		}

		return nil
	},
}
