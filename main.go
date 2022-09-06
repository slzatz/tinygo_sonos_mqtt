package main

/*
combines mqtt and writing to the Adafruit featherwind OLED 128 x 64 display
that uses the sh1107 driver
receives mqtt messages from sonos_scrobble3.py
*/

import (
	"image/color"
	"machine"

	"math/rand"
	"strings"
	"time"

	"github.com/mailru/easyjson"
	"tinygo.org/x/drivers/net/mqtt"
	"tinygo.org/x/drivers/sh1107"
	"tinygo.org/x/drivers/wifinina"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/proggy"
)

var NINA_SPI = machine.SPI0

const (
	NINA_SDO    machine.Pin = machine.PB23
	NINA_SDI    machine.Pin = machine.PB22
	NINA_CS     machine.Pin = machine.PA23
	NINA_SCK    machine.Pin = machine.PA17
	NINA_GPIO0  machine.Pin = machine.PA20
	NINA_RESETN machine.Pin = machine.PA22
	NINA_ACK    machine.Pin = machine.PA21
	NINA_TX     machine.Pin = machine.PB16
	NINA_RX     machine.Pin = machine.PB17
)

var (
	spi     = NINA_SPI
	adaptor *wifinina.Device
	cl      mqtt.Client

	display sh1107.Device
	white   = color.RGBA{255, 255, 255, 255}
	//black   = color.RGBA{1, 1, 1, 255}
	font  = &proggy.TinySZ8pt7b
	Board string
)

// NINA-m4 express pins
const topic = "sonos/current_track"

//easyjson:json
type JSONData struct {
	Artist string
	Title  string
}

func subHandler(client mqtt.Client, msg mqtt.Message) {
	d := &JSONData{}
	err := easyjson.Unmarshal(msg.Payload(), d)
	if err != nil {
		println("easyjson.Unmarshal: ", err)
	}
	println("artist: ", d.Artist)
	println("track: ", d.Title)
	display.ClearDisplay()
	display.ClearBuffer()
	time.Sleep(3000 * time.Millisecond) // needs min ~3 sec
	var line int16
	line = writeString(d.Artist, 22, 50)
	_ = writeString(d.Title, 22, line-20)
	display.Display()
}

func writeString(s string, ln int, line int16) int16 {
	if len(s) < ln {
		tinyfont.WriteLineRotated(&display, font, line, 0, s, white, tinyfont.ROTATION_90)
		return line
	} else {
		ss := strings.Split(s, " ")
		n := len(ss) - len(ss)/2
		firstLine := strings.Join(ss[:n], " ")
		secondLine := strings.Join(ss[n:], " ")
		tinyfont.WriteLineRotated(&display, font, line, 0, firstLine, white, tinyfont.ROTATION_90)
		line -= 15
		tinyfont.WriteLineRotated(&display, font, line, 0, secondLine, white, tinyfont.ROTATION_90)
		return line
	}
}

func main() {
	err := machine.I2C0.Configure(machine.I2CConfig{
		// I think these are the defaults
		Frequency: machine.TWI_FREQ_400KHZ,
		SCL:       machine.SCL_PIN,
		SDA:       machine.SDA_PIN,
	})
	//err := machine.I2C0.Configure(machine.I2CConfig{})
	if err != nil {
		println("could not configure I2C:", err)
		return
	}
	time.Sleep(5 * time.Millisecond)
	display = sh1107.New(machine.I2C0, 0x3C, false)
	display.Configure()
	display.ClearDisplay()

	// Configure SPI for 8Mhz, Mode 0, MSB First
	// using default pins so spi.Configure({machine.SPIConfig{}) should be fine
	spi.Configure(machine.SPIConfig{
		Frequency: 8 * 1e6,
		SDO:       NINA_SDO, //MOSI = machine.SPIO_SDO_PIN
		SDI:       NINA_SDI, //MISO = machine.SPIO_SDI_PIN
		SCK:       NINA_SCK, //SCK = machine.SPIO_SCK_PIN
	})

	time.Sleep(5 * time.Second)
	// Init wifit
	adaptor = wifinina.New(spi,
		NINA_CS,
		NINA_ACK,
		NINA_GPIO0,
		NINA_RESETN,
	)
	//adaptor.Configure()
	adaptor.Configure2(false)   //true = reset active high
	time.Sleep(5 * time.Second) // necessary
	s, err := adaptor.GetFwVersion()
	if err != nil {
		println("GetFwVersion Error:", err)
	}
	println("firmware:", s)

	//time.Sleep(10 * time.Second) ///////

	for {
		err := connectToAP()
		if err == nil {
			break
		}
	}

	opts := mqtt.NewClientOptions()
	clientID := "tinygo-client-" + randomString(len(Board))
	opts.AddBroker(server).SetClientID(clientID)
	println(clientID)
	//opts.AddBroker(server).SetClientID("tinygo-client-2")

	println("Connecting to MQTT broker at", server)
	cl = mqtt.NewClient(opts)
	token := cl.Connect()

	if token.Wait() && token.Error() != nil {
		failMessage("mqtt connect", token.Error().Error())
	}

	// subscribe
	println("Subscribing ...")
	token = cl.Subscribe(topic, 0, subHandler)
	token.Wait()
	if token.Error() != nil {
		failMessage("mqtt subscribe", token.Error().Error())
	}

	for {
		token := cl.Pingreq()
		if token.Error() != nil {
			failMessage("ping", token.Error().Error())
		}
		time.Sleep(30 * time.Second)
	}
}

func failMessage(action, msg string) {
	println(action, ": ", msg)
	time.Sleep(5 * time.Second)
}

func connectToAP() error {
	time.Sleep(2 * time.Second)
	println("Connecting to " + ssid)
	err := adaptor.ConnectToAccessPoint(ssid, pass, 10*time.Second)
	if err != nil {
		println(err)
		return err
	}

	println("Connected.")

	time.Sleep(2 * time.Second)
	ip, _, _, err := adaptor.GetIP()
	for ; err != nil; ip, _, _, err = adaptor.GetIP() {
		println(err.Error())
		time.Sleep(1 * time.Second)
	}
	println(ip.String())
	return nil
}

// Returns an int >= min, < max
func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// Generate a random string of A-Z chars with len = l
func randomString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(randomInt(65, 90))
	}
	return string(bytes)
}
