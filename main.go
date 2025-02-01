package main

import (
	"flag"
	"log"
	"mysql-archiver/config"
	"mysql-archiver/database"
)

var (
	cfgPath string
)

func main() {

	flag.StringVar(&cfgPath, "c", "config.yaml", "special config file")
	flag.Parse()

	app_cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Println("加载配置失败!", err)
		return
	}

	if err := config.ValidConfig(app_cfg); err != nil {
		log.Println("配置校验失败:", err)
		return
	}

	srcDs, dstDs, err := database.InitDataSource(app_cfg)
	if err != nil {
		log.Println("数据源初始化失败! 错误:", err)
		return
	}

	defer func() {
		dstDs.Close()
		srcDs.Close()
	}()

	Archive(app_cfg, srcDs, dstDs)

}
