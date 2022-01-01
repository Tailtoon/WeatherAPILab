package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

var rdb *redis.Client
var ctx = context.Background()

type cachedCityInfo struct {
	CityName  string
	CityTemp  float64
	FeelsLike float64
	Pressure  int
	Humidity  int
	WSpeed    float64
}

func (m cachedCityInfo) MarshalBinary() ([]byte, error) {
	return json.Marshal(m)
}

func weatherByCity(c *gin.Context) {
	cityName := c.Query("city")

	log.Print("Recieved city = ", cityName)

	if cityName == "" {
		// NoCity declared
		log.Print("Warning: No city given.")
		c.Redirect(302, "/")
		return
	}

	rval, err := rdb.Get(ctx, cityName).Result() // Trying to get hashed data
	if err == redis.Nil {
		fmt.Println("REDIS: City not found.")
	} else if err != nil {
		fmt.Println("REDIS Error:", err)
	} else {
		fmt.Println("REDIS: Used cached info")
		fmt.Println(rval)
		var cachedJSON cachedCityInfo

		err = json.Unmarshal([]byte(rval), &cachedJSON)
		if err != nil {
			log.Print("Error: JSON parse error", err)
			c.Redirect(302, "/")
			return
		}
		c.HTML(
			http.StatusOK,
			"index.html",
			gin.H{
				"start":     false,
				"city_name": cachedJSON.CityName,
				"temp":      cachedJSON.CityTemp,
				"flike":     cachedJSON.FeelsLike,
				"pressure":  cachedJSON.Pressure,
				"humidity":  cachedJSON.Humidity,
				"wspeed":    cachedJSON.WSpeed,
			},
		)
		return
	}

	// If city not found in redis cache
	// go get temp from OW
	adr := url.URL{
		Scheme: "https",
		Host:   "api.openweathermap.org",
		Path:   "data/2.5/weather",
		RawQuery: url.Values{
			"q":     {cityName},
			"appid": {os.Getenv("API_KEY")},
			"units": {"metric"},
			"mode":  {"json"},
			"lang":  {"ru"},
		}.Encode(),
	}
	resp, err := http.Get(adr.String())
	if err != nil {
		log.Print("Error: Can't GET weather", err)
		c.Redirect(302, "/")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error: Body read error", err)
		c.Redirect(302, "/")
		return
	}
	log.Printf("Recieved JSON: \n%s", body)

	var recivedJSON OWJSON

	err = json.Unmarshal(body, &recivedJSON)
	if err != nil {
		log.Print("Error: JSON parse error", err)
		c.Redirect(302, "/")
		return
	}

	switch v := recivedJSON.Cod.(type) {
	case string:
		log.Print("Error: ", v, " City not found")
		c.Redirect(http.StatusFound, "/")
		return
	}

	//before we finish
	//lets save data in redis cache
	err = rdb.Set(ctx, cityName, cachedCityInfo{recivedJSON.Name,
		recivedJSON.Main.Temp,
		recivedJSON.Main.FeelsLike,
		recivedJSON.Main.Pressure,
		recivedJSON.Main.Humidity,
		recivedJSON.Wind.Speed}, time.Minute/2).Err()
	if err != nil {
		fmt.Println("REDIS Error:", err)
	}

	c.HTML(
		http.StatusOK,
		"index.html",
		gin.H{
			"start":     false,
			"city_name": recivedJSON.Name,
			"temp":      recivedJSON.Main.Temp,
			"flike":     recivedJSON.Main.FeelsLike,
			"pressure":  recivedJSON.Main.Pressure,
			"humidity":  recivedJSON.Main.Humidity,
			"wspeed":    recivedJSON.Wind.Speed,
		},
	)
}

func mainPage(c *gin.Context) {
	c.HTML(
		http.StatusOK,
		"index.html",
		gin.H{
			"start": true,
		},
	)
}

func serverBegin() {
	// Load env's from file
	godotenv.Load(".env")

	router := gin.Default()

	rdb = redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	router.LoadHTMLGlob("Static/templates/*")
	router.Static("/static/css", "./Static/css")
	router.Static("/static/images", "./Static/images")
	router.StaticFile("/favicon.ico", "./Static/favicon.ico")
	router.GET("/", mainPage)
	router.GET("/weather", weatherByCity)
	router.Run(":80")
}

func main() {
	serverBegin()
}
