package otgorm

import (
	"fmt"
	"time"

	"github.com/DoNewsCode/core/contract"
	"github.com/DoNewsCode/core/di"
	"github.com/DoNewsCode/core/logging"
	"github.com/go-kit/kit/log"
	"github.com/spf13/cobra"
)

const defaultInterval = 15 * time.Second

// MigrationProvider is an interface for database migrations. modules
// implementing this interface are migration providers. migrations will be
// collected in migrate command.
type MigrationProvider interface {
	ProvideMigration() []*Migration
}

// SeedProvider is an interface for database seeding. modules
// implementing this interface are seed providers. seeds will be
// collected in seed command.
type SeedProvider interface {
	ProvideSeed() []*Seed
}

// Module is the registration unit for package core. It provides migration and seed command.
type Module struct {
	maker     Maker
	env       contract.Env
	logger    log.Logger
	container contract.Container
	interval  time.Duration
}

// ModuleIn contains the input parameters needed for creating the new module.
type ModuleIn struct {
	di.In

	Maker     Maker
	Env       contract.Env
	Logger    log.Logger
	Container contract.Container
	Conf      contract.ConfigAccessor
}

// New creates a Module.
func New(in ModuleIn) Module {
	var duration time.Duration = defaultInterval
	in.Conf.Unmarshal("gormMetrics.interval", &duration)
	return Module{
		maker:     in.Maker,
		env:       in.Env,
		logger:    in.Logger,
		container: in.Container,
		interval:  duration,
	}
}

// ProvideCommand provides migration and seed command.
func (m Module) ProvideCommand(command *cobra.Command) {
	var (
		force      bool
		rollbackId string
		logger     = logging.WithLevel(m.logger)
	)
	var migrateCmd = &cobra.Command{
		Use:   "migrate [database]",
		Short: "Migrate gorm tables",
		Long:  `Run all gorm table migrations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var connection = "default"
			if len(args) > 0 {
				connection = args[0]
			}

			if m.env.IsProduction() && !force {
				e := fmt.Errorf("migrations and rollback in production requires force flag to be set")
				return e
			}

			migrations := m.collectMigrations(connection)

			if rollbackId != "" {
				if err := migrations.Rollback(rollbackId); err != nil {
					return fmt.Errorf("unable to rollback: %w", err)
				}

				logger.Info("rollback successfully completed")
				return nil
			}

			if err := migrations.Migrate(); err != nil {
				return fmt.Errorf("unable to migrate: %w", err)
			}

			logger.Info("migration successfully completed")
			return nil
		},
	}
	migrateCmd.Flags().BoolVarP(&force, "force", "f", false, "migrations and rollback in production requires force flag to be set")
	migrateCmd.Flags().StringVarP(&rollbackId, "rollback", "r", "", "rollback to the given migration id")
	migrateCmd.Flag("rollback").NoOptDefVal = "-1"

	var seedCmd = &cobra.Command{
		Use:   "seed [database]",
		Short: "seed the database",
		Long:  `use the provided seeds to bootstrap fake data in database`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var connection = "default"
			if len(args) > 0 {
				connection = args[0]
			}

			if m.env.IsProduction() && !force {
				return fmt.Errorf("seeding in production requires force flag to be set")
			}

			seeds := m.collectSeeds(connection)

			if err := seeds.Seed(); err != nil {
				return fmt.Errorf("seed failed: %w", err)
			}

			logger.Info("seeding successfully completed")
			return nil
		},
	}
	seedCmd.Flags().BoolVarP(&force, "force", "f", false, "seeding in production requires force flag to be set")

	var databaseCmd = &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "manage database",
		Long:    "manage database, such as running migrations",
	}
	databaseCmd.AddCommand(migrateCmd, seedCmd)
	command.AddCommand(databaseCmd)
}

func (m Module) collectMigrations(connection string) Migrations {
	if connection == "" {
		connection = "default"
	}

	var migrations Migrations
	for _, m := range m.container.Modules() {
		if p, ok := m.(MigrationProvider); ok {
			for _, migration := range p.ProvideMigration() {
				if migration.Connection == "" {
					migration.Connection = "default"
				}
				if migration.Connection == connection {
					migrations.Collection = append(migrations.Collection, migration)
				}
			}
		}
	}

	migrations.Db, _ = m.maker.Make(connection)
	return migrations
}

func (m Module) collectSeeds(connection string) Seeds {
	if connection == "" {
		connection = "default"
	}

	var seeds Seeds
	for _, m := range m.container.Modules() {
		if p, ok := m.(SeedProvider); ok {
			for _, seed := range p.ProvideSeed() {
				if seed.Connection == "" {
					seed.Connection = "default"
				}
				if seed.Connection == connection {
					seeds.Collection = append(seeds.Collection, seed)
				}
			}
		}
	}
	seeds.Logger = m.logger
	seeds.Db, _ = m.maker.Make(connection)
	return seeds
}
