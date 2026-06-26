package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	gosocial "github.com/cuvou/gosocial/pkg"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/models/backfill"
	"github.com/cuvou/gosocial/pkg/models/deletion"
	"github.com/cuvou/gosocial/pkg/models/exporting"
	"github.com/cuvou/gosocial/pkg/redis"
	"github.com/cuvou/gosocial/pkg/worker"
	"github.com/urfave/cli/v2"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Build-time values.
var (
	Build     = "n/a"
	BuildDate = "n/a"
)

func init() {
	config.RuntimeVersion = gosocial.Version
	config.RuntimeBuild = Build
	config.RuntimeBuildDate = BuildDate
}

func main() {
	app := &cli.App{
		Name:  "gosocial",
		Usage: "a niche social networking webapp",
		Commands: []*cli.Command{
			{
				Name:  "web",
				Usage: "start the web server",
				Flags: []cli.Flag{
					// Debug mode.
					&cli.BoolFlag{
						Name:    "debug",
						Aliases: []string{"d"},
						Usage:   "debug mode (logging and reloading templates)",
					},

					// HTTP settings.
					&cli.StringFlag{
						Name:    "host",
						Aliases: []string{"H"},
						Value:   "0.0.0.0",
						Usage:   "host address to listen on",
					},
					&cli.IntFlag{
						Name:    "port",
						Aliases: []string{"P"},
						Value:   8080,
						Usage:   "port number to listen on",
					},
				},
				Action: func(c *cli.Context) error {
					if c.Bool("debug") {
						config.Debug = true
						log.SetDebug(true)
					}

					initdb(c)
					initcache(c)

					log.Debug("Debug logging enabled.")

					app := &gosocial.WebServer{
						Host: c.String("host"),
						Port: c.Int("port"),
					}

					// Kick off background worker threads.
					worker.LaunchGoroutines()

					return app.Run()
				},
			},
			{
				Name:  "user",
				Usage: "manage user accounts such as to create admins",
				Subcommands: []*cli.Command{
					{
						Name:  "add",
						Usage: "add a new user account",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "username",
								Aliases:  []string{"u"},
								Required: true,
								Usage:    "username, case insensitive",
							},
							&cli.StringFlag{
								Name:     "email",
								Aliases:  []string{"e"},
								Required: true,
								Usage:    "email address",
							},
							&cli.StringFlag{
								Name:     "password",
								Aliases:  []string{"p"},
								Required: true,
								Usage:    "set user password",
							},
							&cli.BoolFlag{
								Name:  "admin",
								Usage: "set admin status",
							},
						},
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Creating user account: %s", c.String("username"))
							user, err := models.CreateUser(
								c.String("username"),
								c.String("email"),
								c.String("password"),
							)

							if err != nil {
								return err
							}

							// Making an admin?
							if c.Bool("admin") {
								log.Warn("Promoting user to admin status")
								user.IsAdmin = true
								user.Save()
							}
							return nil
						},
					},
					{
						Name:  "export",
						Usage: "create a data export ZIP from a user's account",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "username",
								Aliases:  []string{"u"},
								Required: true,
								Usage:    "username or e-mail, case insensitive",
							},
							&cli.StringFlag{
								Name:     "output",
								Aliases:  []string{"o"},
								Required: true,
								Usage:    "output file (.zip extension)",
							},
						},
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Creating data export for user account: %s", c.String("username"))
							user, err := models.FindUsernameOrEmail(c.String("username"))
							if err != nil {
								return err
							}

							err = exporting.ExportUser(user, c.String("output"))
							return err
						},
					},
					{
						Name:  "delete",
						Usage: "delete one or many user accounts",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: "perform a dry run and do not actually delete any users",
							},
							&cli.BoolFlag{
								Name: "force-dangerously",
								Usage: "by default a STDIN prompt will ask before running in non-dry-run mode, " +
									"pass this setting to skip the prompt and go ahead with deletion (dangerous!)",
							},
							&cli.StringFlag{
								Name:    "username",
								Aliases: []string{"u"},
								Usage:   "delete a specific username OR email address",
							},
							&cli.BoolFlag{
								Name:  "many",
								Usage: "delete many users; required if using the subsequent filter parameters",
							},
							&cli.StringFlag{
								Name:  "status",
								Usage: "only delete accounts having this status (one of: active, disabled, banned)",
							},
							&cli.TimestampFlag{
								Name:   "after",
								Usage:  "delete accounts created after the given datetime (in DateTime '2006-01-02 15:04:05' format)",
								Layout: time.DateTime,
							},
							&cli.TimestampFlag{
								Name:   "last-login-before",
								Usage:  "delete accounts who last logged in before the given datetime ('2006-01-02 15:04:05' format)",
								Layout: time.DateTime,
							},
						},
						Action: func(c *cli.Context) error {
							initdb(c)

							// Either a specific user, or many users.
							var (
								username = c.String("username")
								many     = c.Bool("many")
								force    = c.Bool("force-dangerously")
							)
							if username != "" && many {
								return errors.New("provide a specific username OR the --many flag, not both")
							} else if username == "" && !many {
								return errors.New("provide a specific username OR the --many flag (with filters)")
							}

							// Do the needful.
							return deletion.DeleteManyUsers(deletion.DeleteManyUsersConfig{
								DryRun:          c.Bool("dry-run"),
								Force:           force,
								Username:        username,
								Many:            many,
								Status:          c.String("status"),
								CreatedAfter:    c.Timestamp("after"),
								LastLoginBefore: c.Timestamp("last-login-before"),
							})

							return nil
						},
					},
				},
			},
			{
				Name:  "setup",
				Usage: "setup and data import functions for the website",
				Subcommands: []*cli.Command{
					{
						Name:  "locations",
						Usage: "import the database of world city locations from simplemaps.com",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "input",
								Aliases:  []string{"i"},
								Required: true,
								Usage:    "the input worldcities.csv from simplemaps, with required headers: id, city, lat, lng, country, iso2",
							},
						},
						Action: func(c *cli.Context) error {
							initdb(c)

							var (
								filename = c.String("input")
								reader   io.Reader
							)

							// If the filename is "-" read from STDIN.
							if filename == "-" {
								reader = os.Stdin
							}

							return models.InitializeWorldCities(filename, reader)
						},
					},
				},
			},
			{
				Name:  "backfill",
				Usage: "One-off maintenance tasks and data backfills for database migrations",
				Subcommands: []*cli.Command{
					{
						Name:  "filesizes",
						Usage: "repopulate Filesizes on all photos and comment_photos which have a zero stored in the DB",
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Running BackfillFilesizes()")
							err := backfill.BackfillFilesizes()
							if err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name:  "photo-counts",
						Usage: "repopulate cached Likes and Comment counts on photos",
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Running BackfillPhotoCounts()")
							err := backfill.BackfillPhotoCounts()
							if err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name:  "privacy-settings",
						Usage: "migrate legacy Privacy Settings from profile fields to new table",
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Running BackfillPrivacySettings()")
							err := backfill.BackfillPrivacySettings()
							if err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name:  "profile-themes",
						Usage: "migrate legacy Profile Themes from profile fields to new table",
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Running BackfillProfileThemes()")
							err := backfill.BackfillProfileThemes()
							if err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name:  "followers",
						Usage: "create initial Follows between friends",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "offset",
								Usage: "specify an offset to skip to, default zero",
							},
						},
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Running BackfillFollowers()")
							err := backfill.BackfillFollowers(c.Int("offset"))
							if err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name:  "unfollows",
						Usage: "migrate the legacy feature to mute specific friends' photo uploads, by unfollowing instead",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "offset",
								Usage: "specify an offset to skip to, default zero",
							},
						},
						Action: func(c *cli.Context) error {
							initdb(c)

							log.Info("Running BackfillUnfollows()")
							err := backfill.BackfillUnfollows(c.Int("offset"))
							if err != nil {
								return err
							}

							return nil
						},
					},
				},
			},
			{
				Name:  "vacuum",
				Usage: "Run database maintenance tasks (clean up broken links, remove orphaned comment photos, etc.) for data consistency.",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "dryrun",
						Usage: "don't actually delete anything",
					},
				},
				Action: func(c *cli.Context) error {
					initdb(c)
					return worker.Vacuum(c.Bool("dryrun"))
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		panic(err)
	}
}

func initdb(c *cli.Context) {
	// Load the settings.json
	config.LoadSettings()

	var gormcfg = &gorm.Config{}
	if c.Bool("debug") {
		gormcfg = &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		}
	}

	// Initialize the database.
	log.Info("Initializing DB")
	if config.Current.Database.IsSQLite {
		db, err := gorm.Open(sqlite.Open(config.Current.Database.SQLite), gormcfg)
		if err != nil {
			panic("failed to open SQLite DB")
		}
		models.DB = db
	} else if config.Current.Database.IsPostgres {
		db, err := gorm.Open(postgres.Open(config.Current.Database.Postgres), gormcfg)
		if err != nil {
			panic(fmt.Sprintf("failed to open Postgres DB: %s", err))
		}
		models.DB = db
	} else {
		log.Fatal("A choice of SQL database is required.")
	}

	// Set connection pooling.
	if config.Current.Database.MaxIdleConns > 0 && config.Current.Database.MaxOpenConns > 0 {
		sqlDB, err := models.DB.DB()
		if err != nil {
			log.Error("Setting connection pooling: couldn't get DB: %s", err)
		} else {
			log.Info(
				"DB: set connection pooling to %d idle/%d open conns",
				config.Current.Database.MaxIdleConns,
				config.Current.Database.MaxOpenConns,
			)
			sqlDB.SetMaxIdleConns(config.Current.Database.MaxIdleConns)
			sqlDB.SetMaxOpenConns(config.Current.Database.MaxOpenConns)
			sqlDB.SetConnMaxLifetime(time.Hour)
		}
	}

	// Auto-migrate the DB.
	models.AutoMigrate()
}

func initcache(c *cli.Context) {
	// Initialize Redis.
	log.Info("Initializing Redis")
	redis.Setup(
		fmt.Sprintf("%s:%d/%d",
			config.Current.Redis.Host,
			config.Current.Redis.Port,
			config.Current.Redis.DB,
		),
	)
}
