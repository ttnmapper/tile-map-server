package main

import (
	"os"
	"time"
	"ttnmapper-tms/types"
)

func GetCacheDurationForZoom(z int) time.Duration {
	if z >= 18 {
		return 0 * time.Second // always redraw
	}
	if z >= 16 {
		return 10 * time.Minute
	}
	if z >= 14 {
		return 60 * time.Minute
	}
	if z >= 12 {
		return 2 * time.Hour
	}
	if z >= 10 {
		return 4 * time.Hour
	}
	if z >= 8 {
		return 8 * time.Hour
	}
	if z >= 6 {
		return 10 * time.Hour
	}
	if z >= 4 {
		return 12 * time.Hour
	}
	//if z <= 2 {
	//	return 14*time.Hour
	//}
	if z >= 0 {
		return 24 * time.Hour
	}
	return 0
}

func getZ19TileRange(xOuter int, yOuter int, z int) (xMin int, yMin int, xMax int, yMax int) {

	/*
		+-----------+-----------+
		|   2x,2y   |  2x+1,2y  |
		+-----------+-----------+
		|  2x,2y+1  | 2x+1,2y+1 |
		+-----------+-----------+
	*/
	xNw := xOuter
	yNw := yOuter
	xSe := xOuter + 1
	ySe := yOuter + 1

	for ; z < 19; z++ {
		xNw *= 2
		yNw *= 2

		xSe *= 2
		ySe *= 2
	}

	return xNw, yNw, xSe, ySe
}

func getZ19TileRangeBuffer(xOuter int, yOuter int, z int, buffer int) (xMin int, yMin int, xMax int, yMax int) {

	/*
		+-----------+-----------+
		|   2x,2y   |  2x+1,2y  |
		+-----------+-----------+
		|  2x,2y+1  | 2x+1,2y+1 |
		+-----------+-----------+
	*/
	// Select one tile size extra to all sides. ie 9 tile
	xNw := xOuter - buffer
	yNw := yOuter - buffer
	xSe := xOuter + 1 + buffer
	ySe := yOuter + 1 + buffer

	for ; z < 19; z++ {
		xNw *= 2
		yNw *= 2

		xSe *= 2
		ySe *= 2
	}

	return xNw, yNw, xSe, ySe
}

func getMaxBucket(gridCell types.GridCell) int {
	maxBucketIndex := 12 // Use NoSignal as default
	maxBucketCount := gridCell.BucketNoSignal

	if gridCell.BucketHigh > maxBucketCount {
		maxBucketCount = gridCell.BucketHigh
		maxBucketIndex = 0
	}
	if gridCell.Bucket100 > maxBucketCount {
		maxBucketCount = gridCell.Bucket100
		maxBucketIndex = 1
	}
	if gridCell.Bucket105 > maxBucketCount {
		maxBucketCount = gridCell.Bucket105
		maxBucketIndex = 2
	}
	if gridCell.Bucket110 > maxBucketCount {
		maxBucketCount = gridCell.Bucket110
		maxBucketIndex = 3
	}
	if gridCell.Bucket115 > maxBucketCount {
		maxBucketCount = gridCell.Bucket115
		maxBucketIndex = 4
	}
	if gridCell.Bucket120 > maxBucketCount {
		maxBucketCount = gridCell.Bucket120
		maxBucketIndex = 5
	}
	if gridCell.Bucket125 > maxBucketCount {
		maxBucketCount = gridCell.Bucket125
		maxBucketIndex = 6
	}
	if gridCell.Bucket130 > maxBucketCount {
		maxBucketCount = gridCell.Bucket130
		maxBucketIndex = 7
	}
	if gridCell.Bucket135 > maxBucketCount {
		maxBucketCount = gridCell.Bucket135
		maxBucketIndex = 8
	}
	if gridCell.Bucket140 > maxBucketCount {
		maxBucketCount = gridCell.Bucket140
		maxBucketIndex = 9
	}
	if gridCell.Bucket145 > maxBucketCount {
		maxBucketCount = gridCell.Bucket145
		maxBucketIndex = 10
	}
	if gridCell.BucketLow > maxBucketCount {
		maxBucketCount = gridCell.BucketLow
		maxBucketIndex = 11
	}

	return maxBucketIndex
}

func CreateDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

// Return all grid cells from database between a range of z19 x and y indexes
func GetGlobalSamplesInRange(xMin int, yMin int, xMax int, yMax int) []types.Sample {
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
		Where("x >= ? AND x <= ? AND y >= ? AND y <= ?", xMin, xMax, yMin, yMax).
		Group("x, y").
		Find(&gridCells)

	// Select all grid cells, so we will have duplicates per x,y for every gateway
	//db.Where("x >= ? AND x <= ? AND y >= ? AND y <= ?", xMin, xMax, yMin, yMax).Find(&gridCells)

	for _, gridCell := range gridCells {
		sample := types.Sample{X: gridCell.X, Y: gridCell.Y, MaxBucketIndex: getMaxBucket(gridCell)}
		samples = append(samples, sample)
	}

	// Prometheus stats
	selectElapsed := time.Since(selectStart)
	promTmsGlobalSelectDuration.Observe(float64(selectElapsed.Nanoseconds()) / 1000.0 / 1000.0) //nanoseconds to milliseconds

	return samples
}

// Samples are group by gateway, so it will sum all antennas
func GetGatewaySamplesInRange(gtw_id string, xMin int, yMin int, xMax int, yMax int) []types.Sample {
	selectStart := time.Now()

	var samples []types.Sample
	var gridCells []types.GridCell

	// Select all grid cells, so we will have duplicates per x,y for every gateway
	db.Table("grid_cells").
		Select("x, y, sum(bucket_high) as bucket_high, "+
			"sum(bucket100) as bucket100, sum(bucket105) as bucket105, "+
			"sum(bucket110) as bucket110, sum(bucket115) as bucket115, "+
			"sum(bucket120) as bucket120, sum(bucket125) as bucket125, "+
			"sum(bucket130) as bucket130, sum(bucket135) as bucket135, "+
			"sum(bucket140) as bucket140, sum(bucket145) as bucket145, "+
			"sum(bucket_low) as bucket_low, sum(bucket_no_signal) as bucket_no_signal").
		Joins("left join antennas on antennas.id = grid_cells.antenna_id").
		Where("antennas.gateway_id = ? AND x >= ? AND x <= ? AND y >= ? AND y <= ?", gtw_id, xMin, xMax, yMin, yMax).
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
