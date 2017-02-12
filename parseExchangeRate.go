package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	stdmath "math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/gxui"
	"github.com/google/gxui/drivers/gl"
	"github.com/google/gxui/math"
	"github.com/google/gxui/samples/flags"
)

type RawData struct {
	Date            string `json:"date"`
	Bank            string `json:"bank"`
	Basecurrency    int    `json:"baseCurrency"`
	Basecurrencylit string `json:"baseCurrencyLit"`
	Exchangerate    []struct {
		Basecurrency   string  `json:"baseCurrency"`
		Currency       string  `json:"currency"`
		Saleratenb     float64 `json:"saleRateNB"`
		Purchaseratenb float64 `json:"purchaseRateNB"`
		Salerate       float64 `json:"saleRate"`
		Purchaserate   float64 `json:"purchaseRate"`
	} `json:"exchangeRate"`
}

type Data struct {
	Salerate     float64
	Purchaserate float64
	Date         string
}

func (t *Data) String() string {
	return t.Date + " " + fmt.Sprint(t.Salerate) +
		" " + fmt.Sprint(t.Purchaserate) + "\n"
}

func appMain(driver gxui.Driver) {
	theme := flags.CreateTheme(driver)
	progressBar := theme.CreateProgressBar()
	progressBar.SetDesiredSize(math.Size{W: 480, H: 20})

	var brush gxui.Brush
	brush.Color = gxui.Red10

	items := []string{"RUB", "EUR", "USD"}
	keystr := items[0]
	cdate, pdate := get_date()

	innerLayout1 := theme.CreateLinearLayout()
	innerLayout1.SetDirection(gxui.LeftToRight)

	innerLayout2 := theme.CreateLinearLayout()
	innerLayout2.SetDirection(gxui.TopToBottom)

	innerLayout3 := theme.CreateLinearLayout()
	innerLayout3.SetDirection(gxui.TopToBottom)
	innerLayout3.SetPadding(math.Spacing{L: 5, T: 5, R: 10, B: 40})
	innerLayout3.SetHorizontalAlignment(gxui.AlignCenter)

	h1 := theme.CreateLinearLayout()
	h1.SetDirection(gxui.LeftToRight)
	h1.SetBackgroundBrush(brush)
	h1.SetPadding(math.Spacing{L: 5, T: 5, R: 10, B: 40})
	label := theme.CreateLabel()
	label.SetText("Введите начальную дату парсинга ")
	textBox1 := theme.CreateTextBox()
	textBox1.SetText(pdate)

	h1.AddChild(label)
	h1.AddChild(textBox1)

	h2 := theme.CreateLinearLayout()
	h2.SetDirection(gxui.LeftToRight)
	h2.SetBackgroundBrush(brush)
	h2.SetPadding(math.Spacing{L: 5, T: 5, R: 10, B: 40})
	label2 := theme.CreateLabel()
	label2.SetText("Введите конечную дату парсинга    ")
	textBox2 := theme.CreateTextBox()
	textBox2.SetText(cdate)

	h2.AddChild(label2)
	h2.AddChild(textBox2)

	button := theme.CreateButton()
	button.SetText("Parse")

	button.OnClick(func(gxui.MouseEvent) {

		if button.IsChecked() {
			createMessage(theme, "Дождитесь окончания парсинга")
			return
		}

		str1, str2 := textBox1.Text(), textBox2.Text()

		if r := getDiff(str1, str2); r >= 0 {
			go save_data(str1, str2, addday(str2), keystr, int(r),
				progressBar, button, driver)
		} else {
			createMessage(theme, "Некорректные даты")
		}

	})

	adapter := gxui.CreateDefaultAdapter()
	adapter.SetItems(items)

	List := theme.CreateList()
	List.SetAdapter(adapter)
	List.SetOrientation(gxui.Vertical)
	List.Select(adapter.ItemAt(0))
	List.OnSelectionChanged(func(item gxui.AdapterItem) {
		keystr = fmt.Sprint(item)
	})

	labelV := theme.CreateLabel()
	labelV.SetText("Доступные валюты:")

	innerLayout3.AddChild(labelV)
	innerLayout3.AddChild(List)
	innerLayout3.AddChild(button)

	innerLayout2.AddChild(h1)
	innerLayout2.AddChild(h2)
	innerLayout1.AddChild(innerLayout2)
	innerLayout1.AddChild(innerLayout3)

	mainLayout := theme.CreateLinearLayout()
	mainLayout.SetVerticalAlignment(gxui.AlignBottom)
	mainLayout.AddChild(innerLayout1)
	mainLayout.AddChild(progressBar)

	window := theme.CreateWindow(500, 180, "SimpleParse")
	window.SetScale(flags.DefaultScaleFactor)
	window.AddChild(mainLayout)
	window.OnClose(driver.Terminate)
	window.SetBackgroundBrush(gxui.Brush{gxui.Gray30})

}

func main() {
	gl.StartDriver(appMain)
}

func save_data(start, end, stop, keystr string, diff int,
	progressBar gxui.ProgressBar, button gxui.Button,
	driver gxui.Driver) {
	if diff == 0 {
		diff = 1
	}
	driver.Call(func() {
		button.SetChecked(true)
	})
	out, err := os.Create(keystr + start + "-" + end + ".txt")
	checkError(err)
	defer out.Close()
	wr := bufio.NewWriter(out)
	var data RawData
	str := start
	progressBar.SetTarget(100)
	count := 0
	for str != stop {
		count++
		resp, err := http.Get("https://api.privatbank.ua/p24api/exchange_rates?json&date=" + str)
		checkError(err)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		checkError(json.Unmarshal(body, &data))

		if data.Exchangerate == nil {
			log.Fatalln("Incorrect date")
		}
		first := transformData(&data, keystr)
		if stdmath.Min(first.Salerate, first.Purchaserate) == 0.0 {
			break
		}
		str = addday(first.Date)
		wr.WriteString(first.String())
		driver.Call(func() {
			progressBar.SetProgress(count * 100 / diff)
		})
	}
	if err := wr.Flush(); err != nil {
		log.Fatal(err)
	}
	driver.Call(func() {
		progressBar.SetProgress(0)
		button.SetChecked(false)
	})
}

func transformData(r *RawData, key string) Data {
	var data Data
	data.Date = r.Date
	for _, v := range r.Exchangerate {
		if temp := v.Currency; temp == key {
			data.Salerate = v.Salerate
			data.Purchaserate = v.Purchaserate
			break
		}
	}
	return data
}

func addday(str string) string {
	t := createDate(str).AddDate(0, 0, 1)
	strday := fmt.Sprint(t.Day())
	strmon := fmt.Sprint(int(t.Month()))
	if len(strday) == 1 {
		strday = "0" + strday
	}
	if len(strmon) == 1 {
		strmon = "0" + strmon
	}
	date := strday + "." + strmon + "." + fmt.Sprint(t.Year())
	return date
}

func get_date() (string, string) {
	date := regexp.MustCompile("[.]").Split(time.Now().Local().Format("01.02.2006"), -1)
	date[0], date[1] = date[1], date[0]
	cur := strings.Join(date, ".")
	str := time.Now().Local().AddDate(-1, 0, 0).Format("01.02.2006")
	date = regexp.MustCompile("[.]").Split(str, -1)
	date[0], date[1] = date[1], date[0]
	return cur, strings.Join(date, ".")
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getDiff(first, second string) float64 {
	A := createDate(first)
	B := createDate(second)
	return B.Sub(A).Hours() / 24
}

func createDate(str string) time.Time {
	slice := regexp.MustCompile(`\.`).Split(str, -1)
	day, _ := strconv.Atoi(slice[0])
	month, _ := strconv.Atoi(slice[1])
	year, _ := strconv.Atoi(slice[2])
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func createMessage(theme gxui.Theme, message string) {
	window := theme.CreateWindow(200, 60, "Message Box")

	Label := theme.CreateLabel()
	Label.SetText(message)

	Button := theme.CreateButton()
	Button.SetText("Ok")
	Button.OnClick(func(ev gxui.MouseEvent) {
		window.Close()
	})

	Layout := theme.CreateLinearLayout()
	Layout.AddChild(Label)
	Layout.AddChild(Button)

	window.AddChild(Layout)
}
