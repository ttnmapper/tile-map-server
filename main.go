package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/lib/pq"
	"github.com/patrickmn/go-cache"
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
	CacheDirCircles string `env:"CACHE_DIR_CIRCLES"`
	CacheDirBlocks  string `env:"CACHE_DIR_BLOCKS"`

	CacheEnabled bool `env:"CACHE_ENABLED"`

	PostgresHost     string `env:"POSTGRES_HOST"`
	PostgresPort     string `env:"POSTGRES_PORT"`
	PostgresUser     string `env:"POSTGRES_USER"`
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	PostgresDatabase string `env:"POSTGRES_DATABASE"`
	PostgresDebugLog bool   `env:"POSTGRES_DEBUG_LOG"`

	ListenAddress string `env:"LISTEN_ADDRESS"`
}

var myConfiguration = Configuration{
	CacheDirCircles: "./tile_cache/global_circles",
	CacheDirBlocks:  "./tile_cache/global_blocks",

	CacheEnabled: false,

	PostgresHost:     "localhost",
	PostgresPort:     "5432",
	PostgresUser:     "username",
	PostgresPassword: "password",
	PostgresDatabase: "database",
	PostgresDebugLog: false,

	ListenAddress: ":8080",
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

	promAntennaCacheItemCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ttnmapper_tms_antenna_cache_size",
		Help: "Size of the memory cache that holds antennas previously read from the database",
	})

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

	antennaLastHeardCache *cache.Cache
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

	// Cache
	antennaLastHeardCache = cache.New(5*time.Minute, 10*time.Minute)

	// Register prometheus stats
	prometheus.MustRegister(promAntennaCacheItemCount)
	prometheus.MustRegister(promTmsRequestDuration)
	prometheus.MustRegister(promTmsGlobalSelectDuration)
	prometheus.MustRegister(promTmsGatewaySelectDuration)

	log.Println("Starting server")
	router := mux.NewRouter().UseEncodedPath() //.StrictSlash(true)
	router.Use(loggingMiddleware)
	router.Use(prometheusMiddleware)
	router.HandleFunc("/", Index)
	router.Handle("/metrics", promhttp.Handler())

	// Tile endpoints
	router.HandleFunc("/circles/network/{network_id}/{z}/{x}/{y}", GetCirclesTile)
	router.HandleFunc("/blocks/network/{network_id}/{z}/{x}/{y}", GetBlocksTile)
	router.HandleFunc("/circles/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", GetCirclesTile)
	router.HandleFunc("/blocks/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}", GetBlocksTile)

	log.Fatal(http.ListenAndServe(myConfiguration.ListenAddress, router))

}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "TMS root")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do stuff here
		log.Println(r.RequestURI)
		// Call the next handler, which can be another middleware in the chain, or the final handler.
		next.ServeHTTP(w, r)
	})
}
