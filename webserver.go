package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	messageType = "MESSAGE"
	channelType = "CHANNEL"
	changeType  = "CHANGE"
	userType    = "NEW_USER"
	privateType = "PMESSAGE"
)

var upgrader = websocket.Upgrader{}
var cs = chatServer{}

func init() {
	cs.channels = append(cs.channels, channel{
		ChannelName: "Default",
		Users:       cs.users,
		Messages:    make([]message, 0),
	})

}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/ws", wsocketHandler)
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("public")))

	logCreator := handlers.LoggingHandler(os.Stdout, router)
	server := http.Server{
		Addr:         "0.0.0.0:3000",
		Handler:      logCreator,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	panic(server.ListenAndServe())
}

func wsocketHandler(w http.ResponseWriter, r *http.Request) {
	var conn, _ = upgrader.Upgrade(w, r, w.Header())
	currentUser := user{
		Connection: conn,
	}
	cs.users = append(cs.users, currentUser)
	cs.addUserToDefaultChannel(currentUser)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := cs.sendChannelsList(conn)
	if err != nil {
		log.Fatal(err)
	}

	conn.SetCloseHandler(func(code int, text string) error {
		println("%v %v \n", code, text)
		return nil
	})

	go func(conn *websocket.Conn) {
		var usrname string
		routineUser := user{
			Username:   usrname,
			Connection: conn,
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				println("upgrader error %s\n" + err.Error())
				conn.Close()
				return
			}

			var messg struct {
				Type    string
				Body    interface{}
				Enduser string
			}

			err = json.Unmarshal(msg, &messg)
			if err != nil {
				log.Println(err)
				return
			}
			println("Message Type: ", messg.Type)
			switch messg.Type {
			case "MESSAGE":
				messageText, ok := messg.Body.(string)
				if !ok {
					println("Not a String dude!")
					continue
				}

				newMessage := message{
					Username: cs.findUserName(routineUser.Connection),
					Text:     messageText,
				}

				targetChannel := cs.findUserChannel(routineUser.Connection)
				targetChannel.Messages = append(targetChannel.Messages, newMessage)
				cs.emitMessages(*targetChannel, newMessage)

			case "PMESSAGE":
				messageText, ok := messg.Body.(string)
				if !ok {
					println("Not a String dude!")
					continue
				}
				println("To: ", messg.Enduser, " From: ", cs.findUserName(routineUser.Connection))
				cs.emitPrivateMSG(messg.Enduser, cs.findUserName(routineUser.Connection), messageText)
			case "NEW_CHANNEL":
				channelName, ok := messg.Body.(string)
				if !ok {
					println("Not a String dude!")
					continue
				}

				newChannel := cs.createChannel(channelName)
				cs.channels = append(cs.channels, *newChannel)
				cs.broadcastChannelsList()

			case "CHANGE":
				newchannelName, ok := messg.Body.(string)
				if !ok {
					println("Not a String dude!")
					continue
				}
				currentUserChannel := cs.findUserChannel(routineUser.Connection)
				nextCh := cs.findChannel(newchannelName)
				actualUsr := cs.catchUser(currentUserChannel, routineUser.Connection)
				cs.addUsertoChannel(nextCh.ChannelName, actualUsr)

			case "BROADCAST":
				bMessage, ok := messg.Body.(string)
				if !ok {
					println("Not a string homie!")
					continue
				}
				broadcastMessage := message{
					Text:     bMessage,
					Username: cs.findUserName(routineUser.Connection),
				}
				cs.emitBroadcast(broadcastMessage)

			case "NEW_USER":
				usrName, ok := messg.Body.(string)
				if !ok {
					println("Not a string holmes")
				}
				cs.addUserName(usrName, routineUser.Connection)
				cs.broadcastUsersList()
			}
		}
	}(conn)
}
