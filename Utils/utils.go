package Utils

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NetworkDelay(bytes int) {
	networkDelayMs := 100 + rand.Intn(40) - 20
	time.Sleep(time.Duration(networkDelayMs) * time.Millisecond)
	time.Sleep(time.Duration(float64(bytes)/2.5) * time.Microsecond)
}

func CountBytes(msg interface{}) int {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("countBytes error: ", err.Error())
		os.Exit(1)
	}
	return len(msgBytes)
}

func Mean(numbers []int64) float64 {
	mean := 0.0
	for _, number := range numbers {
		mean += float64(number)
	}
	return mean / float64(len(numbers))
}

func StdDev(numbers []int64) float64 {
	mean := Mean(numbers)
	total := 0.0
	for _, number := range numbers {
		total += math.Pow(float64(number)-mean, 2)
	}
	variance := total / float64(len(numbers)-1)
	return math.Sqrt(variance)
}

func Encode(data interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
