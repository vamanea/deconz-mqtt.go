package main

/*
import "github.com/eclipse/paho.mqtt.golang"
*/

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"log"
	"time"
	"encoding/json"

	"github.com/fatih/color"
	"golang.org/x/net/websocket"
	"github.com/eclipse/paho.mqtt.golang"
)

// Version is the current version.
const Version = "0.1.0"

type deCONZMessage struct {
	e  string
	id string
	r string
	state map[string]interface{}
	t string
}

var (
	origin             string
	url                string
	protocol           string
	displayHelp        bool
	displayVersion     bool
	insecureSkipVerify bool
	red                = color.New(color.FgRed).SprintFunc()
	magenta            = color.New(color.FgMagenta).SprintFunc()
	green              = color.New(color.FgGreen).SprintFunc()
	yellow             = color.New(color.FgYellow).SprintFunc()
	cyan               = color.New(color.FgCyan).SprintFunc()
	wg                 sync.WaitGroup
)

func init() {
	flag.StringVar(&origin, "origin", "http://localhost/", "origin of WebSocket client")
	flag.StringVar(&url, "url", "ws://raspbeegw.lan:8088", "WebSocket server address to connect to")
	flag.StringVar(&protocol, "protocol", "", "WebSocket subprotocol")
	flag.BoolVar(&insecureSkipVerify, "insecureSkipVerify", false, "Skip TLS certificate verification")
	flag.BoolVar(&displayHelp, "help", false, "Display help information about wsd")
	flag.BoolVar(&displayVersion, "version", false, "Display version number")
}

func inLoop(ws *websocket.Conn, errors chan<- error, in chan<- []byte) {
	var msg = make([]byte, 1024)

	for {
		var n int
		var err error

		n, err = ws.Read(msg)

		if err != nil {
			errors <- err
			continue
		}

		in <- msg[:n]
	}
}

func printErrors(errors <-chan error) {
	for err := range errors {
		if err == io.EOF {
			fmt.Printf("\r✝ %v - connection closed by remote\n", magenta(err))
			os.Exit(0)
		} else {
			fmt.Printf("\rerr %v\n> ", red(err))
		}
		os.Exit(-1)
	}
}

func printReceivedMessages(in <-chan []byte) {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "bla:", 0)
	//opts := mqtt.NewClientOptions().AddBroker("tcp://openhab.lan:1883").SetClientID("gotrivial")
	opts := mqtt.NewClientOptions().AddBroker("tcp://openhab.lan:1883")
	opts.SetKeepAlive(2 * time.Second)
	//opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	token := c.Connect()
	if token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Printf("received message\n")
	for msg := range in {
		var f map[string]interface{}
		var subject string
		var text string
		if token.Error() != nil {
			panic(token.Error())
		}
		fmt.Printf("\r< %s\n", cyan(string(msg)))

		err := json.Unmarshal(msg, &f)
		if err != nil {
			fmt.Printf("\r failed to decode JSON - %v\n", red(err))
			continue
		}

		resource := f["r"].(string)
		if resource != "sensors" {
			fmt.Printf("\r not a sensor message - %v\n", red(resource))
			//continue
		}
		fmt.Printf("\r\tresource type is %s\n ", green(string(resource)))

		event := f["e"].(string)
		fmt.Printf("\r\tevent is %s\n ", green(string(event)))

		id := f["id"].(string)
		fmt.Printf("\r\tobject id is %s\n ", green(string(id)))

		if f["state"] == nil {
			continue
		}

		state := f["state"].(map[string]interface{})

		var buttonevent float64
		if state["buttonevent"] != nil {
			buttonevent = state["buttonevent"].(float64)
			fmt.Printf("\r\t\tbutton event is %s\n ", green(buttonevent))
			subject = fmt.Sprintf("deconz/button/%s/buttonevent", id)
			text = fmt.Sprintf("%4.0f", buttonevent)
		}
		var publish bool
		var presence bool
		var temperature float64
		var humidity float64
		if state["presence"] != nil {
			presence = state["presence"].(bool)
			fmt.Printf("\r\t\tpresence event is %s\n ", green(presence))
			subject = fmt.Sprintf("deconz/sensor/%s/presence", id)
			text = fmt.Sprintf("%t", presence)
			publish = true
		} else if state["temperature"] != nil {
			temperature = state["temperature"].(float64) / 100
			fmt.Printf("\r\t\ttemperature event is %s\n ", green(temperature))
			subject = fmt.Sprintf("deconz/sensor/%s/temperature", id)
			text = fmt.Sprintf("%f", temperature)
			publish = true
		} else if state["humidity"] != nil {
			humidity = state["humidity"].(float64) / 100
			fmt.Printf("\r\t\thumidity event is %s\n ", green(humidity))
			subject = fmt.Sprintf("deconz/sensor/%s/humidity", id)
			text = fmt.Sprintf("%f", humidity)
			publish = true
		} else {
			publish = false
		}
		fmt.Printf("\r< %s\n", cyan(string(msg)))

		//text := fmt.Sprintf("this is msg #%s!", resource)
		if publish == true {
			fmt.Printf("\r\tpublish %s to %s\n", green(text), yellow(subject))
			token := c.Publish(subject, 0, false, text)
			token.Wait()
		}
	}
}
/*
func forwardMQTTMessage(in <-chan map[string]interface{}) {
	for msg := range in {
		for k, v := range msg {
		    switch vv := v.(type) {
		    case string:
			fmt.Println(k, "is string", vv)
		    case int:
			fmt.Println(k, "is int", vv)
		    case []interface{}:
			fmt.Println(k, "is an array:")
			for i, u := range vv {
			    fmt.Println(i, u)
			}
		    default:
			fmt.Println(k, "is of a type I don't know how to handle")
		    }
		}


	}
}*/

func dial(url, protocol, origin string) (ws *websocket.Conn, err error) {
	config, err := websocket.NewConfig(url, origin)
	if err != nil {
		return nil, err
	}
	if protocol != "" {
		config.Protocol = []string{protocol}
	}
	config.TlsConfig = &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
	}
	return websocket.DialConfig(config)
}

func main() {
	flag.Parse()

	if displayVersion {
		fmt.Fprintf(os.Stdout, "%s version %s\n", os.Args[0], Version)
		os.Exit(0)
	}

	if displayHelp {
		fmt.Fprintf(os.Stdout, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	ws, err := dial(url, protocol, origin)

	if protocol != "" {
		fmt.Printf("connecting to %s via %s from %s...\n", yellow(url), yellow(protocol), yellow(origin))
	} else {
		fmt.Printf("connecting to %s from %s...\n", yellow(url), yellow(origin))
	}

	defer ws.Close()

	if err != nil {
		panic(err)
	}

	fmt.Printf("successfully connected to %s\n\n", green(url))

	wg.Add(3)

	errors := make(chan error)
	in := make(chan []byte)
	out := make(chan []byte)
	//mqtt := make(chan map[string]interface{})

	defer close(errors)
	defer close(out)
	defer close(in)

	go inLoop(ws, errors, in)
	go printReceivedMessages(in)
	//go forwardMQTTMessage(mqtt)
	go printErrors(errors)

	wg.Wait()
}
