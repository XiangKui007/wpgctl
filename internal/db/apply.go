// Package db 实现智慧水厂 SQL 执行与台账（方案 §6.8）。
package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/wpg/wpgctl/internal/config"
	"github.com/wpg/wpgctl/internal/util"
)

// 文件命名强制：V{version}_{seq}__{desc}.sql
var reSQLName = regexp.MustCompile(`^V([0-9]+\.[0-9]+\.[0-9]+(?:[._-][a-zA-Z0-9._-]+)?)_(\d+)__(.+)\.sql$`)

// Options SQL 执行选项。
type Options struct {
	Site       *config.SiteConfig
	PackageDir string
	DryRun     bool
}

// Script 待执行脚本描述。
type Script struct {
	Path     string
	Version  string
	Seq      string
	Desc     string
	Checksum string
	Database string
}

// Result 执行结果。
type Result struct {
	Applied []string
	Skipped []string
}

// Run 扫描包内 SQL 并按台账幂等执行。
func Run(opts Options) (*Result, error) {
	if opts.Site == nil {
		return nil, fmt.Errorf("site 不能为空")
	}
	if opts.Site.Middleware.MySQL.Disabled {
		util.Warnf("site 已标记跳过 MySQL，db apply 跳过")
		return &Result{}, nil
	}
	sqlRoot := filepath.Join(opts.PackageDir, "sql")
	if !util.DirExists(sqlRoot) {
		util.Warnf("包内无 sql/ 目录，跳过")
		return &Result{}, nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?parseTime=true&multiStatements=true",
		opts.Site.Middleware.MySQL.User,
		opts.Site.Middleware.MySQL.Password,
		opts.Site.Middleware.MySQL.Host,
		opts.Site.Middleware.MySQL.Port,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	res := &Result{}
	err = filepath.Walk(sqlRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(info.Name(), ".sql") {
			return nil
		}
		base := info.Name()
		m := reSQLName.FindStringSubmatch(base)
		if m == nil {
			return fmt.Errorf("SQL 文件名不符合规范 V{ver}_{seq}__{desc}.sql: %s", base)
		}
		sum, err := fileChecksum(path)
		if err != nil {
			return err
		}
		// 数据库名取父目录名
		dbName := filepath.Base(filepath.Dir(path))
		script := Script{
			Path: path, Version: m[1], Seq: m[2], Desc: m[3],
			Checksum: sum, Database: dbName,
		}
		return applyOne(db, script, opts.DryRun, res)
	})
	if err != nil {
		return res, err
	}
	util.Successf("SQL 执行完成：applied=%d skipped=%d", len(res.Applied), len(res.Skipped))
	return res, nil
}

func applyOne(db *sql.DB, s Script, dryRun bool, res *Result) error {
	if _, err := db.Exec("CREATE DATABASE IF NOT EXISTS `" + s.Database + "` DEFAULT CHARSET utf8mb4"); err != nil {
		return err
	}
	if _, err := db.Exec(fmt.Sprintf("USE `%s`", s.Database)); err != nil {
		return err
	}
	if err := ensureHistoryTable(db); err != nil {
		return err
	}

	var existSum string
	err := db.QueryRow(
		`SELECT checksum FROM wpg_deploy_history WHERE filename=?`, filepath.Base(s.Path),
	).Scan(&existSum)
	if err == nil {
		if existSum != s.Checksum {
			return fmt.Errorf("SQL 已执行但 checksum 变更，禁止覆盖: %s（旧=%s 新=%s）",
				filepath.Base(s.Path), existSum, s.Checksum)
		}
		res.Skipped = append(res.Skipped, filepath.Base(s.Path))
		util.Infof("跳过已执行: %s", filepath.Base(s.Path))
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}

	if dryRun {
		util.Infof("[dry-run] 将执行: %s", filepath.Base(s.Path))
		res.Applied = append(res.Applied, filepath.Base(s.Path))
		return nil
	}

	body, err := os.ReadFile(s.Path)
	if err != nil {
		return err
	}
	start := time.Now()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(string(body)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("执行 %s 失败: %w（DDL 可能无法回滚，请按备份指引恢复）", filepath.Base(s.Path), err)
	}
	_, err = tx.Exec(
		`INSERT INTO wpg_deploy_history(filename, checksum, version, executed_at, duration_ms, success, message)
		 VALUES(?,?,?,?,?,?,?)`,
		filepath.Base(s.Path), s.Checksum, s.Version, time.Now(), time.Since(start).Milliseconds(), 1, s.Desc,
	)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	res.Applied = append(res.Applied, filepath.Base(s.Path))
	util.Successf("已执行: %s (%s)", filepath.Base(s.Path), time.Since(start).Round(time.Millisecond))
	return nil
}

func ensureHistoryTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS wpg_deploy_history (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		filename VARCHAR(255) NOT NULL UNIQUE,
		checksum CHAR(64) NOT NULL,
		version VARCHAR(64) NOT NULL,
		executed_at DATETIME NOT NULL,
		duration_ms BIGINT NOT NULL,
		success TINYINT NOT NULL,
		message VARCHAR(512) NULL
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func fileChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ListPending 列出将执行的脚本（供预览）。
func ListPending(packageDir string) ([]Script, error) {
	var list []Script
	sqlRoot := filepath.Join(packageDir, "sql")
	if !util.DirExists(sqlRoot) {
		return list, nil
	}
	err := filepath.Walk(sqlRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".sql") {
			return err
		}
		m := reSQLName.FindStringSubmatch(info.Name())
		if m == nil {
			return fmt.Errorf("非法 SQL 文件名: %s", info.Name())
		}
		sum, _ := fileChecksum(path)
		list = append(list, Script{
			Path: path, Version: m[1], Seq: m[2], Desc: m[3],
			Checksum: sum, Database: filepath.Base(filepath.Dir(path)),
		})
		return nil
	})
	sort.Slice(list, func(i, j int) bool {
		if list[i].Version == list[j].Version {
			return list[i].Seq < list[j].Seq
		}
		return list[i].Version < list[j].Version
	})
	return list, err
}
