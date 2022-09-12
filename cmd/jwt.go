package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

var (
	jwtSecret string
)

func loadconf(configspath string, filename string) {
	if configspath == "" {
		configspath = filepath.Dir("./configs/")
	}
	fileformat := "toml"
	log.Printf("configspath: %s/%s format: %s\n", configspath, filename, fileformat)
	viper.AddConfigPath(configspath)
	viper.SetConfigName(filename)
	viper.SetConfigType(fileformat)
	viper.ReadInConfig()
	jwtSecret = viper.GetString("JWT_SECRET")
	log.Printf("load jwtsecret:%s\n", jwtSecret)
}

func main() {
	var configpath string
	var conffilename string
	var isAdmin = flag.Bool("admin", false, "is Admin?")
	flag.StringVar(&configpath, "confpath", "/etc/dfserver", "configurate file path")
	flag.StringVar(&conffilename, "conf", "config.toml", "configurate file name")
	uuidWithHyphen := uuid.New()

	flag.Parse()
	loadconf(configpath, conffilename)
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)

	claims["id"] = uuidWithHyphen
	claims["admin"] = *isAdmin
	claims["exp"] = time.Now().Add(time.Hour * 24 * 365 * 100).Unix()
	fmt.Println(claims)
	t, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("JWT Token:")
		fmt.Println(t)
	}
}
