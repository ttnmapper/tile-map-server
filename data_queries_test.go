package main

import (
	"github.com/jinzhu/gorm"
	"github.com/patrickmn/go-cache"
	"github.com/tkanos/gonfig"
	"log"
	"testing"
	"time"
)

func initDb() {
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
	//defer db.Close()

	//if myConfiguration.PostgresDebugLog {
	db.LogMode(true)
	//}
}

func TestGetNetworkSamplesInRange(t *testing.T) {
	initDb()

	// https://tile.openstreetmap.org/14/9050/9835.png - Stellebosch Central
	// https://stamen-tiles-d.a.ssl.fastly.net/toner-lite/12/2170/1345.png - wolfsburg central
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(2170, 1345, 12, 0)
	//networkId := "thethingsnetwork.org"
	networkId := "NS_CHIRP://wolfsburg.digital"
	//networkId := "NS_TTS_V3://ttn@000013"
	data := GetNetworkSamplesInRange(networkId, xMin, yMin, xMax, yMax)
	log.Println(data)
}

func TestGetGatewaySamplesInRange(t *testing.T) {
	initDb()

	// https://tile.openstreetmap.org/14/9050/9835.png - Stellebosch Central
	xMin, yMin, xMax, yMax := GetZ19TileRangeBuffer(9050, 9835, 14, 0)
	gatewayId := "eui-60c5a8fffe761551"
	networkId := "thethingsnetwork.org"
	//networkId := "NS_TTS_V3://ttn@000013"
	data := GetGatewaySamplesInRange(networkId, gatewayId, xMin, yMin, xMax, yMax)
	log.Println(data)
}

func TestGetAntennaOnline(t *testing.T) {
	initDb()
	antennaLastHeardCache = cache.New(1*time.Hour, 2*time.Hour)

	log.Println(GetAntennaOnline(1200))
	log.Println(GetAntennaOnline(1200))
}
