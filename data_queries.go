package main

import (
	"github.com/patrickmn/go-cache"
	"strconv"
	"time"
	"ttnmapper-tms/types"
)

// Return all grid cells from database between a range of z19 x and y indexes
//func GetGlobalSamplesInRange(xMin int, yMin int, xMax int, yMax int) []types.Sample {
//	selectStart := time.Now()
//
//	var samples []types.Sample
//	var gridCells []types.GridCell
//
//	// Group by x and y and sum all buckets
//	db.Table("grid_cells").
//		Select("x, y, sum(bucket_high) as bucket_high, "+
//			"sum(bucket100) as bucket100, sum(bucket105) as bucket105, "+
//			"sum(bucket110) as bucket110, sum(bucket115) as bucket115, "+
//			"sum(bucket120) as bucket120, sum(bucket125) as bucket125, "+
//			"sum(bucket130) as bucket130, sum(bucket135) as bucket135, "+
//			"sum(bucket140) as bucket140, sum(bucket145) as bucket145, "+
//			"sum(bucket_low) as bucket_low, sum(bucket_no_signal) as bucket_no_signal").
//		Where("x >= ? AND x <= ? AND y >= ? AND y <= ?", xMin, xMax, yMin, yMax).
//		Group("x, y").
//		Find(&gridCells)
//
//	// Select all grid cells, so we will have duplicates per x,y for every gateway
//	//db.Where("x >= ? AND x <= ? AND y >= ? AND y <= ?", xMin, xMax, yMin, yMax).Find(&gridCells)
//
//	for _, gridCell := range gridCells {
//		sample := types.Sample{X: gridCell.X, Y: gridCell.Y, MaxBucketIndex: getMaxBucket(gridCell)}
//		samples = append(samples, sample)
//	}
//
//	// Prometheus stats
//	selectElapsed := time.Since(selectStart)
//	promTmsGlobalSelectDuration.Observe(float64(selectElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds
//
//	return samples
//}

// Return all grid cells from database between a range of z19 x and y indexes
func GetNetworkSamplesInRange(networkId string, xMin int, yMin int, xMax int, yMax int) []types.Sample {
	selectStart := time.Now()

	var samples []types.Sample
	var gridCells []types.GridCell

	// Group by x and y and sum all buckets
	db.Table("grid_cells").
		Select("antenna_id, x, y, sum(bucket_high) as bucket_high, "+
			"sum(bucket100) as bucket100, sum(bucket105) as bucket105, "+
			"sum(bucket110) as bucket110, sum(bucket115) as bucket115, "+
			"sum(bucket120) as bucket120, sum(bucket125) as bucket125, "+
			"sum(bucket130) as bucket130, sum(bucket135) as bucket135, "+
			"sum(bucket140) as bucket140, sum(bucket145) as bucket145, "+
			"sum(bucket_low) as bucket_low, sum(bucket_no_signal) as bucket_no_signal").
		Joins("left join antennas on antennas.id = grid_cells.antenna_id").
		Where("antennas.network_id= ?", networkId).
		Where("x >= ? AND x <= ? AND y >= ? AND y <= ?", xMin, xMax, yMin, yMax).
		Group("antenna_id, x, y").
		Find(&gridCells)

	for _, gridCell := range gridCells {
		if GetAntennaOnline(gridCell.AntennaID) {
			sample := types.Sample{X: gridCell.X, Y: gridCell.Y, MaxBucketIndex: getMaxBucket(gridCell)}
			samples = append(samples, sample)
		}
	}

	// Prometheus stats
	selectElapsed := time.Since(selectStart)
	promTmsGlobalSelectDuration.Observe(float64(selectElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds

	return samples
}

// Samples are group by gateway, so it will sum all antennas
func GetGatewaySamplesInRange(networkId string, gatewayId string, xMin int, yMin int, xMax int, yMax int) []types.Sample {
	selectStart := time.Now()

	var samples []types.Sample
	var gridCells []types.GridCell

	// Group by x and y and sum all buckets
	db.Table("grid_cells").
		Select("x, y, sum(bucket_high) as bucket_high, "+
			"sum(bucket100) as bucket100, sum(bucket105) as bucket105, "+
			"sum(bucket110) as bucket110, sum(bucket115) as bucket115, "+
			"sum(bucket120) as bucket120, sum(bucket125) as bucket125, "+
			"sum(bucket130) as bucket130, sum(bucket135) as bucket135, "+
			"sum(bucket140) as bucket140, sum(bucket145) as bucket145, "+
			"sum(bucket_low) as bucket_low, sum(bucket_no_signal) as bucket_no_signal").
		Joins("left join antennas on antennas.id = grid_cells.antenna_id").
		Where("antennas.network_id= ?", networkId).
		Where("antennas.gateway_id = ?", gatewayId).
		Where("x >= ? AND x <= ? AND y >= ? AND y <= ?", xMin, xMax, yMin, yMax).
		Group("x, y").
		Find(&gridCells)

	for _, gridCell := range gridCells {
		sample := types.Sample{X: gridCell.X, Y: gridCell.Y, MaxBucketIndex: getMaxBucket(gridCell)}
		samples = append(samples, sample)
	}

	// Prometheus stats
	selectElapsed := time.Since(selectStart)
	promTmsGatewaySelectDuration.Observe(float64(selectElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds

	return samples
}

func GetAntennaOnline(antennaId uint) bool {

	fiveDaysAgo := time.Now().AddDate(0, 0, -5)

	if lastHeardTime, ok := antennaLastHeardCache.Get(strconv.Itoa(int(antennaId))); ok {
		//log.Println("Antenna last heard from cache")
		if lastHeardTime.(time.Time).Before(fiveDaysAgo) {
			return false
		} else {
			return true
		}
	}
	//log.Println("Antenna last heard from db")

	type Result struct {
		LastHeard time.Time
	}

	var result Result
	db.Table("antennas").
		Select("last_heard").
		Joins("JOIN gateways g on antennas.gateway_id = g.gateway_id and antennas.network_id = g.network_id").
		Where("antennas.id= ?", antennaId).
		Scan(&result)

	// Store in cache
	antennaLastHeardCache.Set(strconv.Itoa(int(antennaId)), result.LastHeard, cache.DefaultExpiration)
	promAntennaCacheItemCount.Set(float64(antennaLastHeardCache.ItemCount()))

	if result.LastHeard.Before(fiveDaysAgo) {
		return false
	} else {
		return true
	}
}
