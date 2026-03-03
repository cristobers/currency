package main

import (
	"fmt"
	"os"
	//  "io"
	//  "reflect"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/bojanz/currency"
)

const apiEnvironmentKeyName = "CURRENCYCONVERTER_FREECURRENCYAPI_KEY"
const ONE_HOUR = 3600

type Arguments struct {
	amount string
	from   string
	to     string
}

func main() {
	arguments, err := parse(os.Args)
	if err != nil {
		fmt.Println("Failed to parse:", err)
		helper()
		os.Exit(1)
	}

	// Check if our API key exists
	_, exists := os.LookupEnv(apiEnvironmentKeyName)
	if !exists {
		fmt.Println("Failed to find the API key")
		os.Exit(1)
	}

	rate, err := getExchangeRate(*arguments)
	if err != nil {
		fmt.Println("Failed to get exchange rate:", err)
		os.Exit(1)
	}
	temp, err := convert(*arguments, rate)
	if err != nil {
		fmt.Println("Failed to convert:", err)
		os.Exit(1)
	}
	outputConvertedCurrency(*arguments, *temp)
}

func helper() {
	fmt.Println("./currency <amount> <from> <to>")
}

func parse(args []string) (*Arguments, error) {
	slicedArgs := args[1:]
	if len(slicedArgs) != 3 {
		return nil, fmt.Errorf("Incorrect number of arguments.")
	}

	slicedArgs = convertArgumentsToUppercase(slicedArgs)

	amnt, f, t := slicedArgs[0], slicedArgs[1], slicedArgs[2]
	return &Arguments{
		amount: amnt,
		from:   f,
		to:     t,
	}, nil
}

func getExchangeRate(arguments Arguments) (string, error) {
	baseUrl := "https://api.freecurrencyapi.com/v1/latest?apikey=%s&base_currency=%s"
	key := os.Getenv(apiEnvironmentKeyName)
	apiUrl := fmt.Sprintf(baseUrl, key, arguments.from)

	// Check if we have this api call cached within /tmp/
	//fileName := fmt.Sprintf("/tmp/%s:%s", arguments.from, arguments.to)
	fileName := fmt.Sprintf("/tmp/%s", arguments.from)
	jsonResponse := map[string]map[string]float64{}

	// Cache the API calls so that we don't hit our rate limit.
	fi, err := os.Stat(fileName)

	fileExists := err == nil
	if fileExists {
		diff := fileModificationTimeDiffCurrentTime(fi)
		if diff >= ONE_HOUR {
			// The rates are old, we should update them.
			resp, err := getExchangeRateAPI(apiUrl)
			defer resp.Body.Close()
			if err != nil {
				return "", err
			}

			err = decodeJsonData(jsonResponse, *resp)
			if err != nil {
				return "", err
			}

			// then Write this out to the file, we already know it exists.
			foo, _ := json.Marshal(jsonResponse)
			ioutil.WriteFile(fileName, foo, 0644)
		} else {
			// File exists, and its modify time is less than one hour.
			// Read the file, get the JSON data and decode it
			b, err := ioutil.ReadFile(fileName)
			if err != nil {
				return "", err
			}
			err = json.Unmarshal(b, &jsonResponse)
		}
	} else {
		// the file doesn't exist, we need to write one :)
		// First grab the exchange rates
		resp, err := getExchangeRateAPI(apiUrl)
		defer resp.Body.Close()
		if err != nil {
			return "", err
		}

		err = decodeJsonData(jsonResponse, *resp)
		if err != nil {
			return "", err
		}

		// then Write this out to the file, we already know it exists.
		marshaledBytes, err := json.Marshal(jsonResponse)
		if err != nil {
			return "", err
		}
		// Make the file here
		newFile, err := os.Create(fileName)
		defer newFile.Close()
		if err != nil {
			return "", err
		}
		nBytes, err := newFile.Write(marshaledBytes)
		if err != nil {
			return "", err
		}
		fmt.Printf("Writing %d bytes\n", nBytes)
	}
	innerJsonResponse := jsonResponse["data"]
	s := fmt.Sprint(innerJsonResponse[arguments.to])
	return s, nil
}

func decodeJsonData(jsonResponse map[string]map[string]float64, resp http.Response) error {
	err := json.NewDecoder(resp.Body).Decode(&jsonResponse)
	if err != nil {
		return err
	}
	return nil
}

func getExchangeRateAPI(apiUrl string) (*http.Response, error) {
	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed to send get request to %s:%s\n", apiUrl, err)
	}
	return resp, nil
}

func fileModificationTimeDiffCurrentTime(fi os.FileInfo) int64 {
	currentUnixTime := time.Time.Unix(time.Now())
	modificationTimeAsUnix := time.Time.Unix(fi.ModTime())
	return currentUnixTime - modificationTimeAsUnix
}

func outputConvertedCurrency(arguments Arguments, amount currency.Amount) {
	message := "%s %s to %s is %s\n"
	roundedCurrency := amount.Round()
	fmt.Printf(message, arguments.amount, arguments.from, arguments.to, roundedCurrency)
}

func convertArgumentsToUppercase(args []string) []string {
	for i := 0; i < len(args); i++ {
		args[i] = strings.ToUpper(args[i])
	}
	return args
}

func convert(arguments Arguments, rate string) (*currency.Amount, error) {
	temp, err := currency.NewAmount("1.00", arguments.from)
	if err != nil {
		fmt.Println("Failed to create a new amount:", err)
		return nil, fmt.Errorf("Failed to create a new amount")
	}
	temp, err = temp.Convert(arguments.to, rate)
	if err != nil {
		return nil, fmt.Errorf("Failed to convert")
	}
	temp, err = temp.Mul(arguments.amount)
	if err != nil {
		return nil, fmt.Errorf("Failed to multiply")
	}
	return &temp, nil
}
