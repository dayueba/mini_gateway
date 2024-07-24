package main

import (
	"flag"
	"github.com/dayueba/mini_gateway/internal"
	"log"

	_ "go.uber.org/automaxprocs"
)

var (
	confFile string // 配置文件路径
)

func initArgs() {
	flag.StringVar(&confFile, "config", "./example-config.yaml", "config file")
	flag.Parse()
}

func main() {
	initArgs()

	conf, err := internal.InitConfig(confFile)
	if err != nil {
		log.Fatalln(err)
	}

	if err = internal.InitConnManager(); err != nil {
		log.Fatalln(err)
	}

	if err = internal.InitMerger(); err != nil {
		log.Fatalln(err)
	}

	// init server
	srv := internal.InitServer(conf)
	if err = srv.Run(); err != nil {
		log.Fatalln(err)
	}
}
