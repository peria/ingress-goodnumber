package ingressgoodnumber

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"

	"appengine"
)

const pageTemplate = `<!doctype html>
<html>
  <head>
    <title>Ingress Good Number</title>
	<link rel="stylesheet" type="text/css" href="/main.css" />
    <script type="text/javascript" src="/main.js"></script>
  </head>
  <body>
    <div id="contents">
      <h1>Ingress Good Number</h1>
      <p>"Ingress Good Number" tells you minimum actions to achieve "good number" APs, such as rounded numbers, straight numbers, and repeated numbers.</p>
      <div id='apinput'>
        Your current AP: <input type="text" id="ap" /><input type="button" value="check" id="checkBtn" onclick="goodnumber.postAp()" />
      </div>
      <div id="result"><div>
	</div>
	<div id="footer-note"><span class="note">Note: </span>If you created double control field by accident, try making additional link, then your actions are counted as creating normal 2 CFs.</div>
  </body>
</html>`

const (
	generatorCap = 10
	pattenrsCap  = 20
)

var apGain = []int64{
	// 2813, // Create double CF
	1750, // Full deploy
	1563, // Create a CF
	1199, // Destroy a CF
	625,  // Capture a portal
	375,  // Complete a portal
	313,  // Create a link
	262,  // Destroy a link
	125,  // Place a resonator or mod
	100,  // Hack ememy portal
	75,   // Destroy a resonator
	65,   // Upgrade others' resonator
	10,   // Recharge a portal
}

// StatusRequest is a struct defining input data.
type StatusRequest struct {
	AP int64 `json:"ap"`
}

// RestActionResponse is a struct defining output data to client.
type RestActionResponse struct {
	Target int64 `json:"target"`
	// CreateDoubleCF int64 `json:"create double control field"`
	FullDeploy    int64 `json:"full deploy"`
	CreateCF      int64 `json:"create control field"`
	DestroyCF     int64 `json:"destroy control field"`
	CapturePortal int64 `json:"capture portal"`
	CompPortal    int64 `json:"complete portal"`
	CreateLink    int64 `json:"create link"`
	DestroyLink   int64 `json:"destroy link"`
	PlaceRes      int64 `json:"place resonator"`
	Hack          int64 `json:"hack portal"`
	DestroyRes    int64 `json:"destroy resonator"`
	UpgradeRes    int64 `json:"upgrade resonator"`
	Recharge      int64 `json:"recharge"`
}

// Define Int64Slice to sort int64 values
type Int64Slice []int64

func (s Int64Slice) Len() int           { return len(s) }
func (s Int64Slice) Less(i, j int) bool { return s[i] < s[j] }
func (s Int64Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func init() {
	http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		postHandler(w, r)
	case "GET":
		getHandler(w, r)
	default:
		fmt.Fprintf(w, "This endpoint only support GET or POST methods")
	}
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, pageTemplate)
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	c := appengine.NewContext(r)

	var status StatusRequest
	err := decoder.Decode(&status)
	if err != nil {
		message := fmt.Sprintf("An error occured during parsing: %v", err)
		http.Error(w, message, 400)
		c.Errorf("%v", message)
		return
	}
	gn := genGoodNumbers(status.AP)
	c.Infof("AP: %v", status.AP)
	target := <-gn

	pattern := findPattern(status.AP, target)
	action := NewRestActionResponse(target, pattern)
	resp, err := json.Marshal(action)
	if err != nil {
		http.Error(w, err.Error(), 500)
		c.Errorf("%v", err.Error())
		return
	}
	c.Infof("%v", string(resp))
	fmt.Fprintf(w, "%v", string(resp))
}

// NewRestActionResponse converts AP list into a struct.
func NewRestActionResponse(target int64, pattern map[int64]int64) *RestActionResponse {
	return &RestActionResponse{
		Target: target,
		// CreateDoubleCF: pattern[2813],
		FullDeploy:    pattern[1750],
		CreateCF:      pattern[1563],
		DestroyCF:     pattern[1199],
		CapturePortal: pattern[625],
		CompPortal:    pattern[375],
		CreateLink:    pattern[313],
		PlaceRes:      pattern[125],
		Hack:          pattern[100],
		DestroyRes:    pattern[75],
		UpgradeRes:    pattern[65],
		Recharge:      pattern[10],
	}
}

func genGoodNumbers(ap int64) <-chan int64 {
	gn := make(chan int64, generatorCap)
	go func(num int64) {
		nearestRound := genRound(num)
		nearestRep := genRepDigit(num)
		nearestSeq := genSeqDigit(num)
		nearestPi := genPiDigit(num)

		nearValues := Int64Slice([]int64{nearestRound, nearestRep, nearestSeq, nearestPi})
		sort.Sort(nearValues)
		for _, value := range nearValues {
			gn <- value
		}
	}(ap)
	return gn
}

// TODO: Speed up this function. (It works fast enough for actual values, though.)
func findPattern(ap, target int64) map[int64]int64 {
	gap := target - ap
	if gap > 100000000 {
		// Do not compute to avoid wasting resources.
		return createCounterMap()
	}
	dp := make([]int64, gap+1)
	track := make([]int64, gap+1)

	// initialize
	for i := int64(0); i <= gap; i++ {
		dp[i] = math.MaxInt64
	}
	dp[0] = 0

	// find solution
	for i := int64(0); i < gap+1; i++ {
		if dp[i] == math.MaxInt64 {
			continue
		}

		step := dp[i] + 1
		for _, n := range apGain {
			k := i + n
			if k <= gap {
				if step < dp[k] {
					dp[k] = step
					track[k] = i
				}
			}
		}
	}

	// find pattern
	result := createCounterMap()
	if dp[gap] != math.MaxInt64 {
		for p := gap; p > 0; p = track[p] {
			result[p-track[p]]++
		}
	}
	return result
}

// genRound returns the minimum rounded number > `ap`
func genRound(ap int64) int64 {
	digit := numDigits(ap)
	round := int64(math.Pow10(digit))
	return ((ap / round) + 1) * round
}

// genRepDigit returns the minimum rep-digit number > `ap`
func genRepDigit(ap int64) int64 {
	// Make a rep-unit number (11...11).
	digit := numDigits(ap)
	ru := int64(1)
	for i := 0; i < digit; i++ {
		ru = ru*10 + 1
	}
	// Change the digits. (22...22, 33...33, ...)
	for i := int64(1); i < 10; i++ {
		val := ru * i
		if val > ap {
			return val
		}
	}
	// In case ap == 99..99, return 111..11.
	return ru*10 + 1
}

// genSeqDigit returns the minimum sequencial digit number > `ap`
func genSeqDigit(ap int64) int64 {
	num := int64(1)
	for i := int64(2); num <= ap; i++ {
		num = num*10 + i%10
	}
	return num
}

// genPiDigit returns the minimum pi-digit number > `ap`, like 314159
func genPiDigit(ap int64) int64 {
	// Maxium candidate in int64
	num := int64(3141592653589793238)
	for num/10 >= ap {
		num = num / 10
	}
	return num
}

// Find order of exponent
func numDigits(num int64) int {
	digit := 0
	for {
		num = num / 10
		if num == 0 {
			break
		}
		digit++
	}
	return digit
}

func createCounterMap() map[int64]int64 {
	counter := make(map[int64]int64)
	for _, k := range apGain {
		counter[k] = int64(0)
	}
	return counter
}
