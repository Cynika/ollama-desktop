package dao

import (
	"context"
	"database/sql"
	"embed"
	_ "github.com/glebarez/go-sqlite"
	"github.com/jianggujin/go-dbfly"
	"ollama-desktop/internal/config"
	"os"
	"path/filepath"
	"time"
)

//go:embed sql
var sqlFiles embed.FS

type DbDao struct {
	ctx context.Context
	db  *sql.DB
}

var Dao *DbDao

func (d *DbDao) Startup(ctx context.Context) {
	d.ctx = ctx
	if err := d.init(); err != nil {
		panic(err)
	}
}

func (d *DbDao) Shutdown() {
	if d.db != nil {
		d.db.Close()
		d.db = nil
	}
}

func (d *DbDao) init() error {
	if d.db != nil {
		return nil
	}
	// 连接到SQLite数据库
	path := config.DbFileName
	dir := filepath.Dir(path)
	os.MkdirAll(dir, os.ModePerm)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	db.SetMaxIdleConns(10)
	// SetMaxOpenConns sets the maximum number of open connections to the database.
	db.SetMaxOpenConns(50)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	db.SetConnMaxLifetime(time.Duration(3600) * time.Second)
	// SetConnMaxIdleTime sets the maximum amount of time a connection may be idle.
	db.SetConnMaxIdleTime(time.Duration(1800) * time.Second)
	d.db = db
	return d.migrate()
}

func (d *DbDao) migrate() error {
	driver := dbfly.NewSqlDriver(d.db)
	fly := dbfly.NewDbfly(dbfly.NewSqliteMigratory(), driver, &dbfly.EmbedFSSource{
		Fs:    sqlFiles,
		Paths: []string{"sql"},
	})
	fly.SetChangeTableName("database_change_log")
	return fly.MigrateContext(d.ctx)
}

func (d *DbDao) GetDb() *sql.DB {
	return d.db
}
