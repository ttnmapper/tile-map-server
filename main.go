package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tkanos/gonfig"
	"log"
	"net/http"
	"time"
)

/*
Technopark https://tile.openstreetmap.org/15/18098/19674.png
*/

type Configuration struct {
	CacheDirCircles string
	CacheDirBlocks  string

	CacheEnabled bool

	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     string `env:"POSTGRES_PORT"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `env:"POSTGRES_DEBUG_LOG"`

	WebservicePort int
}

var myConfiguration = Configuration{
	CacheDirCircles: "./heatmapTiles",
	CacheDirBlocks:  "./blocksTiles",

	CacheEnabled: true,

	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	WebservicePort: 8000,
}

var (
	promTmsRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ttnmapper_tms_request_duration",
		Help:    "Duration to serve a request",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	},
		[]string{"path"},
	)

	//promTmsCirclesCacheCount = promauto.NewCounter(prometheus.CounterOpts{
	//	Name: "ttnmapper_tms_circles_cache_count",
	//	Help: "The number of circles tiles served from cache",
	//})
	//promTmsCirclesCreateCount = promauto.NewCounter(prometheus.CounterOpts{
	//	Name: "ttnmapper_tms_circles_create_count",
	//	Help: "The number of new circles tiles created",
	//})
	//
	//promTmsCirclesDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	//	Name:    "ttnmapper_tms_circles_create_duration",
	//	Help:    "Duration of creating a circles tile",
	//	Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	//})
	//
	//promTmsBlocksCacheCount = promauto.NewCounter(prometheus.CounterOpts{
	//	Name: "ttnmapper_tms_blocks_cache_count",
	//	Help: "The number of blocks tiles served from cache",
	//})
	//promTmsBlocksCreateCount = promauto.NewCounter(prometheus.CounterOpts{
	//	Name: "ttnmapper_tms_blocks_create_count",
	//	Help: "The number of new blocks tiles created",
	//})
	//
	//promTmsBlocksDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	//	Name:    "ttnmapper_tms_blocks_create_duration",
	//	Help:    "Duration of creating a blocks tile",
	//	Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	//})

	promTmsGlobalSelectDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ttnmapper_tms_select_global_duration",
		Help:    "Duration of selecting global data for one tile from the database",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	})
	promTmsGatewaySelectDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ttnmapper_tms_select_gateway_duration",
		Help:    "Duration of selecting gateway data for one tile from the database",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.5, 2, 5, 10, 100, 1000, 10000},
	})

	// Other global vars
	db *gorm.DB
)

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()

		startTime := time.Now()

		next.ServeHTTP(w, r)

		elapsedTime := time.Since(startTime)
		promTmsRequestDuration.WithLabelValues(path).Observe(float64(elapsedTime.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
	})
}

func main() {

	err := gonfig.GetConf("conf.json", &myConfiguration)
	if err != nil {
		log.Println(err)
	}

	log.Printf("[Configuration]\n%s\n", prettyPrint(myConfiguration)) // output: [UserA, UserB]

	// Table name prefixes
	gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
		//return "ttnmapper_" + defaultTableName
		return defaultTableName
	}

	var dbErr error
	// pq: unsupported sslmode "prefer"; only "require" (default), "verify-full", "verify-ca", and "disable" supported - so we disable it
	db, dbErr = gorm.Open("postgres", "host="+myConfiguration.PostgresHost+" port="+myConfiguration.PostgresPort+" user="+myConfiguration.PostgresUser+" dbname="+myConfiguration.PostgresDatabase+" password="+myConfiguration.PostgresPassword+" sslmode=disable")
	if dbErr != nil {
		log.Println("Error connecting to Postgres")
		panic(dbErr.Error())
	}
	defer db.Close()

	if myConfiguration.PostgresDebugLog {
		db.LogMode(true)
	}

	// Register prometheus stats
	prometheus.MustRegister(promTmsRequestDuration)
	prometheus.MustRegister(promTmsGlobalSelectDuration)
	prometheus.MustRegister(promTmsGatewaySelectDuration)

	log.Println("Starting server")
	router := mux.NewRouter().StrictSlash(true)
	router.Use(prometheusMiddleware)
	router.HandleFunc("/", Index)
	router.Handle("/metrics", promhttp.Handler())

	// Tile endpoints
	router.HandleFunc("/circles/{z}/{x}/{y}", GetCirclesTile)
	router.HandleFunc("/blocks/{z}/{x}/{y}", GetBlocksTile)
	router.HandleFunc("/gateway-circles/{gateway}/{z}/{x}/{y}", GetCirclesTile)
	router.HandleFunc("/gateway-blocks/{gateway}/{z}/{x}/{y}", GetBlocksTile)

	listenInfo := fmt.Sprintf("0.0.0.0:%d", myConfiguration.WebservicePort)
	log.Fatal(http.ListenAndServe(listenInfo, router))

}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "TMS root")
}
