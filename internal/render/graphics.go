package render

import (
	_ "embed"
	"fmt"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
)

//go:embed fonts/LiberationSans-Regular.ttf
var regularFontData []byte

//go:embed fonts/LiberationSans-Bold.ttf
var boldFontData []byte

const (
	colorWhite = "#ffffff"
	colorBlack = "#343a40"
	colorRed   = "#dc3545"
	colorGrey  = "#6c757d"
)

var (
	regularFont *truetype.Font
	boldFont    *truetype.Font
)

func init() {
	var err error
	regularFont, err = truetype.Parse(regularFontData)
	if err != nil {
		panic(fmt.Sprintf("failed to parse regular font: %v", err))
	}
	boldFont, err = truetype.Parse(boldFontData)
	if err != nil {
		panic(fmt.Sprintf("failed to parse bold font: %v", err))
	}
}

type calendarRenderer struct {
	dc     *gg.Context
	width  int
	height int
}

func newCalendarRenderer(width, height int) *calendarRenderer {
	dc := gg.NewContext(width, height)
	dc.SetHexColor(colorWhite)
	dc.Clear()
	return &calendarRenderer{
		dc:     dc,
		width:  width,
		height: height,
	}
}

func (r *calendarRenderer) drawHeader(data TemplateData) {
	headerHeight := 60.0
	padding := 24.0

	r.dc.SetHexColor(colorGrey)
	r.dc.DrawLine(0, headerHeight, float64(r.width), headerHeight)
	r.dc.SetLineWidth(2)
	r.dc.Stroke()

	r.dc.SetHexColor(colorBlack)
	r.dc.SetFontFace(truetype.NewFace(boldFont, &truetype.Options{Size: 28}))
	title := fmt.Sprintf("%s %d", data.MonthName, data.Year)
	r.dc.DrawString(title, padding, 40)

	r.dc.SetFontFace(truetype.NewFace(regularFont, &truetype.Options{Size: 12}))
	r.dc.SetHexColor(colorGrey)
	generatedText := fmt.Sprintf("Generated: %s | Battery: %s", data.GeneratedAt, data.BatteryPercentage)
	textWidth, _ := r.dc.MeasureString(generatedText)
	r.dc.DrawString(generatedText, float64(r.width)-padding-textWidth, 35)

	if data.WeatherError != "" {
		r.dc.SetHexColor(colorRed)
		errorWidth, _ := r.dc.MeasureString(data.WeatherError)
		r.dc.DrawString(data.WeatherError, float64(r.width)-padding-errorWidth, 50)
	}
}

func (r *calendarRenderer) drawWeekdayHeaders(y float64) float64 {
	weekdays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	headerHeight := 35.0
	colWidth := float64(r.width) / 7.0

	r.dc.SetHexColor(colorGrey)
	r.dc.DrawLine(0, y+headerHeight, float64(r.width), y+headerHeight)
	r.dc.SetLineWidth(2)
	r.dc.Stroke()

	r.dc.SetHexColor(colorBlack)
	r.dc.SetFontFace(truetype.NewFace(boldFont, &truetype.Options{Size: 13}))
	for i, day := range weekdays {
		x := float64(i)*colWidth + 12
		r.dc.DrawString(day, x, y+22)

		if i < 6 {
			r.dc.SetHexColor(colorGrey)
			lineX := float64(i+1) * colWidth
			r.dc.DrawLine(lineX, y, lineX, y+headerHeight)
			r.dc.SetLineWidth(1)
			r.dc.Stroke()
			r.dc.SetHexColor(colorBlack)
		}
	}

	return y + headerHeight
}

func (r *calendarRenderer) drawCalendarGrid(data TemplateData, startY float64) {
	numWeeks := len(data.Weeks)
	if numWeeks == 0 {
		return
	}

	colWidth := float64(r.width) / 7.0
	rowHeight := (float64(r.height) - startY) / float64(numWeeks)

	for weekIdx, week := range data.Weeks {
		rowY := startY + float64(weekIdx)*rowHeight

		for dayIdx, day := range week.Days {
			cellX := float64(dayIdx) * colWidth
			cellY := rowY

			r.drawDay(day, cellX, cellY, colWidth, rowHeight)

			r.dc.SetHexColor(colorGrey)
			if dayIdx < 6 {
				r.dc.DrawLine(cellX+colWidth, cellY, cellX+colWidth, cellY+rowHeight)
				r.dc.SetLineWidth(1)
				r.dc.Stroke()
			}
		}

		if weekIdx < numWeeks-1 {
			r.dc.SetHexColor(colorGrey)
			r.dc.DrawLine(0, rowY+rowHeight, float64(r.width), rowY+rowHeight)
			r.dc.SetLineWidth(1)
			r.dc.Stroke()
		}
	}
}

func (r *calendarRenderer) drawDay(day DayData, x, y, width, height float64) {
	padding := 10.0

	dayNumColor := colorBlack
	if !day.IsCurrentMonth {
		dayNumColor = colorGrey
	}

	if day.IsToday {
		r.dc.SetHexColor(colorRed)
		circleX := x + padding + 16
		circleY := y + 8 + 16
		r.dc.DrawCircle(circleX, circleY, 16)
		r.dc.Fill()
		dayNumColor = colorWhite
	}

	r.dc.SetHexColor(dayNumColor)
	r.dc.SetFontFace(truetype.NewFace(regularFont, &truetype.Options{Size: 18}))
	r.dc.DrawString(day.DayNum, x+padding+6, y+12+18)

	if day.DayNum == "1" {
		r.dc.SetFontFace(truetype.NewFace(boldFont, &truetype.Options{Size: 12}))
		r.dc.SetHexColor(colorBlack)
		r.dc.DrawString(day.MonthShort, x+padding+36, y+8+18)
	}

	if day.DayTemp != "" {
		r.dc.SetFontFace(truetype.NewFace(regularFont, &truetype.Options{Size: 13}))
		r.dc.SetHexColor(colorBlack)
		dayTempWidth, _ := r.dc.MeasureString(day.DayTemp)
		r.dc.DrawString(day.DayTemp, x+width-padding-dayTempWidth, y+padding+11)

		r.dc.SetHexColor(colorGrey)
		nightTempWidth, _ := r.dc.MeasureString(day.NightTemp)
		r.dc.DrawString(day.NightTemp, x+width-padding-nightTempWidth, y+padding+24)
	}

	r.drawEvents(day, x, y+40, width, height-40, day.IsPast)
}

func (r *calendarRenderer) drawEvents(day DayData, x, y, width, height float64, isPast bool) {
	if len(day.Events) == 0 {
		return
	}

	eventHeight := 22.0
	gap := 2.0
	padding := 6.0

	r.dc.SetFontFace(truetype.NewFace(regularFont, &truetype.Options{Size: 13}))

	currentY := y
	for _, event := range day.Events {
		if currentY+eventHeight > y+height {
			break
		}

		if event.AllDay {
			bgColor := colorBlack
			if isPast {
				bgColor = colorGrey
			}
			r.dc.SetHexColor(bgColor)
			r.dc.DrawRoundedRectangle(x+padding, currentY, width-2*padding, eventHeight, 3)
			r.dc.Fill()

			r.dc.SetHexColor(colorWhite)
			availableWidth := width - 2*padding - 12
			truncatedSummary := r.truncateText(event.Summary, availableWidth)
			r.dc.DrawString(truncatedSummary, x+padding+6, currentY+16)
		} else {
			timeColor := colorRed
			titleColor := colorBlack
			if isPast {
				timeColor = colorGrey
				titleColor = colorGrey
			}

			r.dc.SetHexColor(timeColor)
			timeText := event.Time
			r.dc.DrawString(timeText, x+padding+6, currentY+16)

			timeWidth, _ := r.dc.MeasureString(timeText)
			r.dc.SetHexColor(titleColor)
			availableWidth := width - padding - 6 - timeWidth - 6 - padding
			truncatedSummary := r.truncateText(event.Summary, availableWidth)
			r.dc.DrawString(truncatedSummary, x+padding+6+timeWidth+6, currentY+16)
		}

		currentY += eventHeight + gap
	}
}

func (r *calendarRenderer) truncateText(text string, maxWidth float64) string {
	textWidth, _ := r.dc.MeasureString(text)
	if textWidth <= maxWidth {
		return text
	}

	ellipsis := "..."
	ellipsisWidth, _ := r.dc.MeasureString(ellipsis)

	if maxWidth <= ellipsisWidth {
		return ellipsis
	}

	for i := len(text); i > 0; i-- {
		truncated := text[:i] + ellipsis
		truncatedWidth, _ := r.dc.MeasureString(truncated)
		if truncatedWidth <= maxWidth {
			return truncated
		}
	}

	return ellipsis
}

func (r *calendarRenderer) savePNG(outputPath string) error {
	return r.dc.SavePNG(outputPath)
}

func RenderCalendarToPNG(data TemplateData, outputPath string) error {
	renderer := newCalendarRenderer(data.Width, data.Height)

	renderer.drawHeader(data)

	weekdayY := renderer.drawWeekdayHeaders(60)

	renderer.drawCalendarGrid(data, weekdayY)

	return renderer.savePNG(outputPath)
}

func RenderErrorToPNG(width, height int, errorMsg string, errorDetails map[string]string, outputPath string) error {
	dc := gg.NewContext(width, height)
	dc.SetHexColor(colorWhite)
	dc.Clear()

	padding := 40.0

	dc.SetHexColor(colorRed)
	dc.DrawRectangle(padding, padding, float64(width)-2*padding, float64(height)-2*padding)
	dc.SetLineWidth(3)
	dc.Stroke()

	dc.SetFontFace(truetype.NewFace(boldFont, &truetype.Options{Size: 32}))
	dc.SetHexColor(colorRed)
	dc.DrawString("Error Generating Calendar", padding+30, padding+60)

	dc.SetFontFace(truetype.NewFace(regularFont, &truetype.Options{Size: 18}))
	dc.SetHexColor(colorBlack)
	dc.DrawStringWrapped(errorMsg, padding+30, padding+120, 0, 0, float64(width)-2*padding-60, 1.5, gg.AlignLeft)

	dc.SetFontFace(truetype.NewFace(regularFont, &truetype.Options{Size: 14}))
	currentY := padding + 220.0
	for key, value := range errorDetails {
		dc.SetHexColor(colorBlack)
		dc.DrawString(fmt.Sprintf("%s:", key), padding+30, currentY)
		dc.SetHexColor(colorGrey)
		dc.DrawString(value, padding+150, currentY)
		currentY += 25
	}

	return dc.SavePNG(outputPath)
}
