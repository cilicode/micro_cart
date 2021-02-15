package main

import (
	"cart/domain/repository"
	service2 "cart/domain/service"
	"cart/handler"
	go_micro_service_cart "cart/proto/cart"
	"github.com/cilicode/common"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/registry"
	consul2 "github.com/micro/go-plugins/registry/consul/v2"
	ratelimit "github.com/micro/go-plugins/wrapper/ratelimiter/uber/v2"
	opentracing2 "github.com/micro/go-plugins/wrapper/trace/opentracing/v2"
	"github.com/opentracing/opentracing-go"
	"log"
)

var QPS = 100

func main() {
	consulConfig, err := common.GetConsulConfig("127.0.0.1", 8500, "/micro/config")

	if err != nil {
		log.Print(err)
	}

	consul := consul2.NewRegistry(func(options *registry.Options) {
		options.Addrs = []string{
			"127.0.0.1:8500",
		}
	})

	//链路追踪
	t, io, err := common.NewTracer("go.micro.service.cart", "localhost:6831")
	if err != nil {
		log.Print(err)
	}
	defer io.Close()
	opentracing.SetGlobalTracer(t)

	//数据库连接
	mysqlInfo := common.GetMysqlFromConsul(consulConfig, "mysql")
	//创建数据库连接
	db, err := gorm.Open("mysql", mysqlInfo.User+":"+mysqlInfo.Pwd+"@/"+mysqlInfo.Database+"?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Print(err)
	}
	defer db.Close()
	//禁止副表
	db.SingularTable(true)

	//第一次初始化
	err = repository.NewCartRepository(db).InitTable()
	if err != nil {
		log.Print(err)
	}

	service := micro.NewService(
		micro.Name("go.micro.service.cart"),
		micro.Version("latest"),
		micro.Address("0.0.0.0:8087"),
		micro.Registry(consul),
		micro.WrapHandler(opentracing2.NewHandlerWrapper(opentracing.GlobalTracer())),
		micro.WrapHandler(ratelimit.NewHandlerWrapper(QPS)),
	)

	service.Init()
	cartDataService := service2.NewCartDataService(repository.NewCartRepository(db))
	go_micro_service_cart.RegisterCartHandler(
		service.Server(),
		&handler.Cart{CartDataService: cartDataService})

	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
