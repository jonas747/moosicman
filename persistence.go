package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
)

type SavedState struct {
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`

	Queue   []string `json:"queue"`
	Shuffle bool     `json:"shuffle"`
}

func LoadPlayersFromDisk(path string) error {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	var decoded []*SavedState
	err = json.Unmarshal(file, &decoded)
	if err != nil {
		return err
	}

	for _, state := range decoded {
		p := GetPlayer(state.GuildID)
		if p != nil {
			continue
		}

		p, err = CreatePlayer(state.GuildID, state.ChannelID)
		if err != nil {
			log.Println("Error creating saved player:", err)
			continue
		}

		if state.Shuffle {
			p.ToggleShuffle()
		}
		p.TogglePersist()
		for _, item := range state.Queue {
			err = p.QueueUp(item)
			if err != nil {
				log.Println("Error queuing up:", err)
			}
		}
	}
	return nil
}

func SavePlayers(path string) error {
	states := make([]*SavedState, 0)

	playersLock.Lock()

	for _, player := range players {
		status := player.Status()
		if !status.Persist {
			continue
		}

		if len(status.Queue) < 1 {
			continue
		}

		channelId := ""
		if player.vc != nil {
			channelId = player.vc.ChannelID
		}
		if channelId == "" {
			continue
		}

		queue := make([]string, len(status.Queue))

		for i, v := range status.Queue {
			queue[i] = v.ID
		}

		state := &SavedState{
			GuildID:   player.guildID,
			ChannelID: channelId,
			Queue:     queue,
			Shuffle:   status.Shuffle,
		}
		states = append(states, state)
	}
	playersLock.Unlock()

	if len(states) < 1 {
		return nil
	}

	out, err := json.Marshal(states)
	if err != nil {
		return err
	}

	log.Println("Saving", len(states), "States")
	err = ioutil.WriteFile(path, out, 0755)
	return err
}

func ListenStopSignal() {
	sigChan := make(chan os.Signal)

	signal.Notify(sigChan, os.Interrupt, os.Kill)

	<-sigChan
	log.Println("Shutting down..")
	err := SavePlayers(PlayersPath)
	if err != nil {
		log.Println("Error saving players:", err)
	}
	os.Exit(0)
}
