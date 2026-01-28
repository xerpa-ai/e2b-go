package e2b

import (
	"encoding/json"
	"fmt"
)

// ChartType represents the type of chart.
type ChartType string

const (
	ChartTypeLine          ChartType = "line"
	ChartTypeScatter       ChartType = "scatter"
	ChartTypeBar           ChartType = "bar"
	ChartTypePie           ChartType = "pie"
	ChartTypeBoxAndWhisker ChartType = "box_and_whisker"
	ChartTypeSuperChart    ChartType = "superchart"
	ChartTypeUnknown       ChartType = "unknown"
)

// ScaleType represents the axis scale type.
type ScaleType string

const (
	ScaleTypeLinear      ScaleType = "linear"
	ScaleTypeDatetime    ScaleType = "datetime"
	ScaleTypeCategorical ScaleType = "categorical"
	ScaleTypeLog         ScaleType = "log"
	ScaleTypeSymlog      ScaleType = "symlog"
	ScaleTypeLogit       ScaleType = "logit"
	ScaleTypeFunction    ScaleType = "function"
	ScaleTypeFunctionLog ScaleType = "functionlog"
	ScaleTypeAsinh       ScaleType = "asinh"
	ScaleTypeUnknown     ScaleType = "unknown"
)

// Chart is the interface for all chart types.
type Chart interface {
	// ChartType returns the type of the chart.
	ChartType() ChartType

	// ChartTitle returns the title of the chart.
	ChartTitle() string

	// ToMap converts the chart to a map for JSON serialization.
	ToMap() map[string]any
}

// BaseChart contains common fields for all chart types.
type BaseChart struct {
	Type    ChartType      `json:"type"`
	Title   string         `json:"title"`
	RawData map[string]any `json:"-"`
}

// ChartType returns the chart type.
func (c *BaseChart) ChartType() ChartType {
	return c.Type
}

// ChartTitle returns the chart title.
func (c *BaseChart) ChartTitle() string {
	return c.Title
}

// ToMap returns the raw data map.
func (c *BaseChart) ToMap() map[string]any {
	return c.RawData
}

// Chart2D contains fields common to 2D charts.
type Chart2D struct {
	BaseChart
	XLabel string `json:"x_label"`
	YLabel string `json:"y_label"`
	XUnit  string `json:"x_unit"`
	YUnit  string `json:"y_unit"`
}

// PointData represents a data series with labeled points.
type PointData struct {
	Label  string  `json:"label"`
	Points []Point `json:"points"`
}

// Point represents a single data point.
type Point struct {
	X any `json:"x"`
	Y any `json:"y"`
}

// PointChart contains fields for point-based charts (line, scatter).
type PointChart struct {
	Chart2D
	XTicks      []any       `json:"x_ticks"`
	XTickLabels []string    `json:"x_tick_labels"`
	XScale      ScaleType   `json:"x_scale"`
	YTicks      []any       `json:"y_ticks"`
	YTickLabels []string    `json:"y_tick_labels"`
	YScale      ScaleType   `json:"y_scale"`
	Elements    []PointData `json:"-"`
}

// LineChart represents a line chart.
type LineChart struct {
	PointChart
}

// ChartType returns the chart type.
func (c *LineChart) ChartType() ChartType {
	return ChartTypeLine
}

// ScatterChart represents a scatter chart.
type ScatterChart struct {
	PointChart
}

// ChartType returns the chart type.
func (c *ScatterChart) ChartType() ChartType {
	return ChartTypeScatter
}

// BarData represents a single bar in a bar chart.
type BarData struct {
	Label string `json:"label"`
	Group string `json:"group"`
	Value string `json:"value"`
}

// BarChart represents a bar chart.
type BarChart struct {
	Chart2D
	Elements []BarData `json:"-"`
}

// ChartType returns the chart type.
func (c *BarChart) ChartType() ChartType {
	return ChartTypeBar
}

// PieData represents a slice in a pie chart.
type PieData struct {
	Label  string  `json:"label"`
	Angle  float64 `json:"angle"`
	Radius float64 `json:"radius"`
}

// PieChart represents a pie chart.
type PieChart struct {
	BaseChart
	Elements []PieData `json:"-"`
}

// ChartType returns the chart type.
func (c *PieChart) ChartType() ChartType {
	return ChartTypePie
}

// BoxAndWhiskerData represents data for a box and whisker plot.
type BoxAndWhiskerData struct {
	Label         string    `json:"label"`
	Min           float64   `json:"min"`
	FirstQuartile float64   `json:"first_quartile"`
	Median        float64   `json:"median"`
	ThirdQuartile float64   `json:"third_quartile"`
	Max           float64   `json:"max"`
	Outliers      []float64 `json:"outliers"`
}

// BoxAndWhiskerChart represents a box and whisker chart.
type BoxAndWhiskerChart struct {
	Chart2D
	Elements []BoxAndWhiskerData `json:"-"`
}

// ChartType returns the chart type.
func (c *BoxAndWhiskerChart) ChartType() ChartType {
	return ChartTypeBoxAndWhisker
}

// SuperChart represents a chart containing multiple sub-charts.
type SuperChart struct {
	BaseChart
	Elements []Chart `json:"-"`
}

// ChartType returns the chart type.
func (c *SuperChart) ChartType() ChartType {
	return ChartTypeSuperChart
}

// DeserializeChart deserializes a chart from a map.
func DeserializeChart(data map[string]any) (Chart, error) {
	if data == nil {
		return nil, nil
	}

	chartType, ok := data["type"].(string)
	if !ok {
		return nil, fmt.Errorf("chart type not found or invalid")
	}

	// Store raw data for serialization
	rawJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chart data: %w", err)
	}

	switch ChartType(chartType) {
	case ChartTypeLine:
		var chart LineChart
		if err := json.Unmarshal(rawJSON, &chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal line chart: %w", err)
		}
		chart.RawData = data
		chart.Elements = parsePointData(data)
		return &chart, nil

	case ChartTypeScatter:
		var chart ScatterChart
		if err := json.Unmarshal(rawJSON, &chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal scatter chart: %w", err)
		}
		chart.RawData = data
		chart.Elements = parsePointData(data)
		return &chart, nil

	case ChartTypeBar:
		var chart BarChart
		if err := json.Unmarshal(rawJSON, &chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bar chart: %w", err)
		}
		chart.RawData = data
		chart.Elements = parseBarData(data)
		return &chart, nil

	case ChartTypePie:
		var chart PieChart
		if err := json.Unmarshal(rawJSON, &chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pie chart: %w", err)
		}
		chart.RawData = data
		chart.Elements = parsePieData(data)
		return &chart, nil

	case ChartTypeBoxAndWhisker:
		var chart BoxAndWhiskerChart
		if err := json.Unmarshal(rawJSON, &chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal box and whisker chart: %w", err)
		}
		chart.RawData = data
		chart.Elements = parseBoxAndWhiskerData(data)
		return &chart, nil

	case ChartTypeSuperChart:
		var chart SuperChart
		if err := json.Unmarshal(rawJSON, &chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal super chart: %w", err)
		}
		chart.RawData = data
		// Parse sub-charts from elements
		if elements, ok := data["elements"].([]any); ok {
			for _, elem := range elements {
				if elemMap, ok := elem.(map[string]any); ok {
					subChart, err := DeserializeChart(elemMap)
					if err == nil && subChart != nil {
						chart.Elements = append(chart.Elements, subChart)
					}
				}
			}
		}
		return &chart, nil

	default:
		// Unknown chart type - return base chart
		var chart BaseChart
		if err := json.Unmarshal(rawJSON, &chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal chart: %w", err)
		}
		chart.Type = ChartTypeUnknown
		chart.RawData = data
		return &chart, nil
	}
}

// parsePointData extracts point data from chart data.
func parsePointData(data map[string]any) []PointData {
	var result []PointData

	elements, ok := data["elements"].([]any)
	if !ok {
		return result
	}

	for _, elem := range elements {
		elemMap, ok := elem.(map[string]any)
		if !ok {
			continue
		}

		pd := PointData{
			Label: getString(elemMap, "label"),
		}

		if points, ok := elemMap["points"].([]any); ok {
			for _, p := range points {
				if pArr, ok := p.([]any); ok && len(pArr) >= 2 {
					pd.Points = append(pd.Points, Point{X: pArr[0], Y: pArr[1]})
				}
			}
		}

		result = append(result, pd)
	}

	return result
}

// parseBarData extracts bar data from chart data.
func parseBarData(data map[string]any) []BarData {
	var result []BarData

	elements, ok := data["elements"].([]any)
	if !ok {
		return result
	}

	for _, elem := range elements {
		elemMap, ok := elem.(map[string]any)
		if !ok {
			continue
		}

		bd := BarData{
			Label: getString(elemMap, "label"),
			Group: getString(elemMap, "group"),
			Value: getStringOrNumber(elemMap, "value"),
		}

		result = append(result, bd)
	}

	return result
}

// parsePieData extracts pie data from chart data.
func parsePieData(data map[string]any) []PieData {
	var result []PieData

	elements, ok := data["elements"].([]any)
	if !ok {
		return result
	}

	for _, elem := range elements {
		elemMap, ok := elem.(map[string]any)
		if !ok {
			continue
		}

		pd := PieData{
			Label:  getString(elemMap, "label"),
			Angle:  getFloat(elemMap, "angle"),
			Radius: getFloat(elemMap, "radius"),
		}

		result = append(result, pd)
	}

	return result
}

// parseBoxAndWhiskerData extracts box and whisker data from chart data.
func parseBoxAndWhiskerData(data map[string]any) []BoxAndWhiskerData {
	var result []BoxAndWhiskerData

	elements, ok := data["elements"].([]any)
	if !ok {
		return result
	}

	for _, elem := range elements {
		elemMap, ok := elem.(map[string]any)
		if !ok {
			continue
		}

		bwd := BoxAndWhiskerData{
			Label:         getString(elemMap, "label"),
			Min:           getFloat(elemMap, "min"),
			FirstQuartile: getFloat(elemMap, "first_quartile"),
			Median:        getFloat(elemMap, "median"),
			ThirdQuartile: getFloat(elemMap, "third_quartile"),
			Max:           getFloat(elemMap, "max"),
		}

		if outliers, ok := elemMap["outliers"].([]any); ok {
			for _, o := range outliers {
				if v, ok := o.(float64); ok {
					bwd.Outliers = append(bwd.Outliers, v)
				}
			}
		}

		result = append(result, bwd)
	}

	return result
}

// getString safely extracts a string from a map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getStringOrNumber extracts a value as string, converting numbers if needed.
func getStringOrNumber(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case int:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// getFloat safely extracts a float64 from a map.
func getFloat(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}
