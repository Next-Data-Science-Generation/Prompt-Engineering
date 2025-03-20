package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"gonum.org/v1/gonum/stat"
)

// Load CSV file
func loadCSV(filename string) [][]string {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error opening CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	data, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Error reading CSV file: %v", err)
	}
	return data
}

// Load Excel file
func loadExcel(filename string) [][]string {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		log.Fatalf("Error opening Excel file: %v", err)
	}

	// Print available sheet names
	sheets := f.GetSheetList()
	fmt.Println("Available Sheets in Excel:", sheets)

	// Use the first sheet automatically
	if len(sheets) == 0 {
		log.Fatalf("No sheets found in the Excel file")
	}
	sheet := sheets[0]
	fmt.Println("Using Sheet:", sheet)

	// Read the sheet data
	rows, err := f.GetRows(sheet)
	if err != nil {
		log.Fatalf("Error reading Excel sheet: %v", err)
	}
	return rows
}

// Function to filter Algeria data
func filterAlgeria(data [][]string, countryCol int) [][]string {
	var result [][]string
	for _, row := range data {
		if len(row) > countryCol && strings.EqualFold(row[countryCol], "Algeria") {
			result = append(result, row)
		}
	}
	return result
}

// Haversine formula to calculate distance (in km)
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // Earth's radius in km
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// Convert string to float safely
func parseFloat(s string) float64 {
	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0.0
	}
	return val
}

// Join datasets within 3km clustering
func joinDatasets(csvData, excelData [][]string, csvLatCol, csvLonCol, excelLatCol, excelLonCol int) ([][]string, [][]string) {
	var joined [][]string
	var dangling [][]string

	for _, csvRow := range csvData[1:] {
		csvLat, csvLon := parseFloat(csvRow[csvLatCol]), parseFloat(csvRow[csvLonCol])
		closestDist := 3.0
		var bestMatch []string

		for _, excelRow := range excelData[1:] {
			excelLat, excelLon := parseFloat(excelRow[excelLatCol]), parseFloat(excelRow[excelLonCol])
			distance := haversine(csvLat, csvLon, excelLat, excelLon)
			if distance < closestDist {
				closestDist = distance
				bestMatch = excelRow
			}
		}

		if bestMatch != nil {
			joinedRow := append(csvRow, bestMatch...)
			joined = append(joined, joinedRow)
		} else {
			dangling = append(dangling, csvRow)
		}
	}

	return joined, dangling
}

// Extract regression data
func extractRegressionData(joinedData [][]string, flaringVolIndex int, independentIndexes []int) ([]float64, [][]float64) {
	var target []float64
	var predictors [][]float64

	for _, row := range joinedData {
		if len(row) > flaringVolIndex {
			y := parseFloat(row[flaringVolIndex])
			target = append(target, y)

			var x []float64
			for _, idx := range independentIndexes {
				if len(row) > idx {
					x = append(x, parseFloat(row[idx]))
				}
			}
			predictors = append(predictors, x)
		}
	}
	return target, predictors
}

// Normalize a slice using Min-Max Scaling
func normalize(data []float64) []float64 {
	minVal, maxVal := data[0], data[0]
	for _, val := range data {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}
	scaled := make([]float64, len(data))
	for i, val := range data {
		scaled[i] = (val - minVal) / (maxVal - minVal)
	}
	return scaled
}

// Perform linear regression with normalization
func runRegression(y []float64, x [][]float64) {
	if len(x) == 0 || len(y) == 0 || len(x[0]) == 0 {
		log.Fatalf("Error: Insufficient data for regression analysis")
	}

	// Normalize x values
	var xFlat []float64
	for _, row := range x {
		xFlat = append(xFlat, row[0])
	}
	xFlat = normalize(xFlat)

	// Print normalized values for debugging
	fmt.Println("\nSample Normalized Data (First 10 values):")
	for i := 0; i < len(y) && i < 10; i++ {
		fmt.Printf("y[%d] (Flaring Volume 2019): %.4f, x[%d] (Normalized Predictor): %.4f\n", i, y[i], i, xFlat[i])
	}

	// Compute regression coefficients
	alpha, beta := stat.LinearRegression(y, xFlat, nil, false)
	fmt.Printf("\nRegression Model (Normalized): Flaring Volume = %.4f + %.4f * Predictor\n", alpha, beta)

	// Compute R-squared
	yMean := stat.Mean(y, nil)
	ssTotal, ssResidual := 0.0, 0.0
	for i := range y {
		predicted := alpha + beta*xFlat[i]
		ssTotal += (y[i] - yMean) * (y[i] - yMean)
		ssResidual += (y[i] - predicted) * (y[i] - predicted)
	}
	rSquared := 1 - (ssResidual / ssTotal)
	fmt.Printf("R-squared (Normalized): %.4f\n", rSquared)
}

func main() {
	// Load datasets
	csvData := loadCSV("eog_global_flare_survey_2015_flare_list.csv")
	excelData := loadExcel("2012-2023-individual-flare-volume-estimates.xlsx")

	// Extract headers
	fmt.Println("CSV Headers:", csvData[0])
	fmt.Println("Excel Headers:", excelData[0])

	// Identify column indexes
	csvCountryIndex, csvLatIndex, csvLonIndex := 0, 4, 5
	excelCountryIndex, excelLatIndex, excelLonIndex, flaringVolIndex := 0, 1, 2, 10 // "Flaring Vol (million m3)"

	// Independent variables
	independentIndexes := []int{6, 7, 8} // "flr_volume", "avg_temp", "dtc_freq"

	// Filter Algeria records
	algeriaCSV := filterAlgeria(csvData, csvCountryIndex)
	algeriaExcel := filterAlgeria(excelData, excelCountryIndex)

	// Print statistics
	fmt.Printf("Filtered Algeria Records in CSV: %d\n", len(algeriaCSV)-1)
	fmt.Printf("Filtered Algeria Records in Excel: %d\n", len(algeriaExcel)-1)

	// Join datasets
	joinedData, _ := joinDatasets(algeriaCSV, algeriaExcel, csvLatIndex, csvLonIndex, excelLatIndex, excelLonIndex)

	// Print merge results
	fmt.Printf("Joined Records (within 3km): %d\n", len(joinedData))

	// Extract regression data
	y, x := extractRegressionData(joinedData, flaringVolIndex, independentIndexes)

	// Run regression analysis
	runRegression(y, x)
}
