package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var Db *sql.DB

func MysqlDbConnect() {
	envError := godotenv.Load(".env")

	if envError != nil {
		log.Fatalf("Error loading .env file")
	}

	cfg := mysql.Config{
		User:                 os.Getenv("DB_USERNAME_PROD"),
		Passwd:               os.Getenv("DB_PASS_PROD"),
		Addr:                 os.Getenv("DB_ADDR_PROD"),
		Net:                  os.Getenv("DB_NET_PROD"),
		DBName:               os.Getenv("DB_DATABASE_PROD"),
		AllowNativePasswords: true,
	}

	var err error

	Db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err.Error())
	}

	pingErr := Db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	fmt.Println("Connected to MySQL DB.")

}

// export DB_USERNAME_PROD=root
// export DB_PASS_PROD=Bonaventure
// export DB_ADDR_PROD=127.0.0.1:3306
// export DB_NET_PROD=tcp
// export DB_DATABASE_PROD=kiron
