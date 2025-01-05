package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"strings"
)

type Member struct {
	Nickname string `json:"nickname"`
	QQ       string `json:"qq"`
	Bili     string `json:"biliuid"`
	MemberID int    `json:"memberid"`
}

func setContentTypeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case strings.HasSuffix(path, ".js"):
			c.Header("Content-Type", "application/javascript")
		case strings.HasSuffix(path, ".css"):
			c.Header("Content-Type", "text/css")
		}
		c.Next()
	}
}

func main() {
	db, err := sql.Open("sqlite3", "./akmembers.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTables(db)

	r := gin.Default()
	r.Use(cors.Default())
	r.Use(setContentTypeMiddleware()) // 添加中间件来设置 MIME 类型

	// API routes
	api := r.Group("/api")
	{
		api.POST("/signup", func(c *gin.Context) {
			var member Member
			if err := c.ShouldBindJSON(&member); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "错误的请求,请联系工作人员"})
				return
			}

			if member.Nickname == "" || member.QQ == "" || member.Bili == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "错误的请求,请联系工作人员"})
				return
			}

			count, err := getCount(db)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -10, "msg": "服务器内部错误"})
				return
			}

			query := "SELECT * FROM Akmembers WHERE QQ = ? OR bili = ?"
			rows, err := db.Query(query, member.QQ, member.Bili)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -10, "msg": "服务器内部错误"})
				return
			}
			defer rows.Close()

			if rows.Next() {
				c.JSON(http.StatusConflict, gin.H{"code": -30, "msg": "您已提交过招新单!"})
				return
			}

			insertQuery := "INSERT INTO Akmembers (nickname, QQ, bili, MemberID) VALUES (?, ?, ?, ?)"
			_, err = db.Exec(insertQuery, member.Nickname, member.QQ, member.Bili, count)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -10, "msg": "服务器内部错误"})
				return
			}

			updateCount(db, count+1)

			c.JSON(http.StatusOK, gin.H{
				"code":     0,
				"msg":      "提交成功,欢迎加入!",
				"memberid": count,
			})
		})

		api.GET("/lookup", func(c *gin.Context) {
			qq := c.Query("qq")
			if qq == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "错误的请求,请联系工作人员"})
				return
			}

			query := `
				SELECT nickname, QQ, MemberID 
				FROM (
					SELECT nickname, QQ, MemberID FROM Akmembers
					UNION
					SELECT nickname, QQ, MemberID FROM another
					UNION
					SELECT nickname, QQ, MemberID FROM rgmembers
				) AS sheetone 
				WHERE QQ = ?
			`
			row := db.QueryRow(query, qq)
			var nickname string
			var memberID int
			err := row.Scan(&nickname, &qq, &memberID)
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"code": -30, "msg": "QQ号不存在于数据库中,请核对"})
				return
			} else if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -10, "msg": "服务器内部错误"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"code":     0,
				"nickname": nickname,
				"memberid": memberID,
			})
		})
	}

	// Serve static files from the Vite build directory
	r.Static("/assets", "./build/assets")

	// Serve index.html for all other routes
	r.NoRoute(func(c *gin.Context) {
		c.File("./build/index.html")
	})

	r.Run(":15680")
}

func createTables(db *sql.DB) {
	createMembersTableSQL := `
	CREATE TABLE IF NOT EXISTS Akmembers (
		nickname TEXT NOT NULL,
		QQ TEXT NOT NULL,
		bili TEXT NOT NULL,
		MemberID INTEGER PRIMARY KEY AUTOINCREMENT
	);
	`
	_, err := db.Exec(createMembersTableSQL)
	if err != nil {
		log.Fatalf("无法创建Akmembers表: %v", err)
	}

	createAnotherTableSQL := `
	CREATE TABLE IF NOT EXISTS another (
		nickname TEXT NOT NULL,
		QQ TEXT NOT NULL,
		bili TEXT NOT NULL,
		MemberID INTEGER PRIMARY KEY AUTOINCREMENT
	);
	`
	_, err = db.Exec(createAnotherTableSQL)
	if err != nil {
		log.Fatalf("无法创建another表: %v", err)
	}

	createRGMembersTableSQL := `
	CREATE TABLE IF NOT EXISTS rgmembers (
		nickname TEXT NOT NULL,
		QQ TEXT NOT NULL,
		bili TEXT NOT NULL,
		MemberID INTEGER PRIMARY KEY AUTOINCREMENT
	);
	`
	_, err = db.Exec(createRGMembersTableSQL)
	if err != nil {
		log.Fatalf("无法创建rgmembers表: %v", err)
	}

	createCountTableSQL := `
	CREATE TABLE IF NOT EXISTS AkCount (
		UIDCount INTEGER PRIMARY KEY
	);
	`
	_, err = db.Exec(createCountTableSQL)
	if err != nil {
		log.Fatalf("无法创建AkCount表: %v", err)
	}

	checkCountRowSQL := `SELECT COUNT(*) FROM AkCount;`
	row := db.QueryRow(checkCountRowSQL)
	var count int
	err = row.Scan(&count)
	if err != nil {
		log.Fatalf("无法查询AkCount表: %v", err)
	}

	if count == 0 {
		initialCountSQL := `INSERT INTO AkCount (UIDCount) VALUES (0);`
		_, err = db.Exec(initialCountSQL)
		if err != nil {
			log.Fatalf("无法初始化AkCount表: %v", err)
		}
	}
}

func getCount(db *sql.DB) (int, error) {
	getCountSQL := `SELECT UIDCount FROM AkCount LIMIT 1;`
	row := db.QueryRow(getCountSQL)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("无法获取计数: %v", err)
	}
	return count, nil
}

func updateCount(db *sql.DB, newCount int) {
	updateCountSQL := `UPDATE AkCount SET UIDCount = ?;`
	_, err := db.Exec(updateCountSQL, newCount)
	if err != nil {
		log.Printf("无法更新计数: %v", err)
	}
}
