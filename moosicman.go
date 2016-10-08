package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	CmdPrefix = ">"
	Version   = "0.0.1"
)

// Variables used for command line parameters
var (
	Token          string
	DiscordSession *discordgo.Session

	ErrNoPlayer = errors.New("No player in this server D:")
)

func init() {
	flag.StringVar(&Token, "t", "", "Account Token")
	flag.Parse()
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if Token == "" {
		log.Fatal("No token provided")
	}

	log.Println("Starting up moosicman version", Version, "by a very beautiful man")

	session, err := discordgo.New(Token)
	checkErr(err)

	DiscordSession = session

	session.AddHandler(handleMessageCreate)
	session.AddHandler(handleReady)
	session.AddHandler(handleGuildCreate)

	session.LogLevel = discordgo.LogInformational

	checkErr(session.Open())

	select {}
}

func handleReady(s *discordgo.Session, m *discordgo.Ready) {
	log.Println("Ready received! Fire away with the commands b0ss")
	log.Println("If this is a bot account people acn invite it with the following link:")
	log.Println("https://discordapp.com/oauth2/authorize?client_id=CLIENT_ID_HERE&scope=bot")
	log.Println("Replace 'CLIENT_ID_HERE' with the bot's client id")
}

func handleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	log.Printf("Joined guild %s (%s)\n", g.Name, g.ID)
}

func handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	split := strings.SplitN(m.Content, " ", 2)

	channel, err := DiscordSession.State.Channel(m.ChannelID)
	if err != nil {
		log.Println("Failed finding channel:", err)
		return
	}

	guild, err := DiscordSession.State.Guild(channel.GuildID)
	if err != nil {
		log.Println("Failed finding guild:", err)
		return
	}

	var vs *discordgo.VoiceState

	for _, v := range guild.VoiceStates {
		if v.UserID == m.Author.ID {
			vs = v
			break
		}
	}

	if vs == nil {
		return
	}

	cmd := strings.ToLower(split[0])

	if cmd == CmdPrefix+"join" {
		log.Println(vs.ChannelID, guild.ID)
		_, err = CreatePlayer(guild.ID, vs.ChannelID)
		if err != nil {
			log.Println("Error creating player:", err)
		} else {
			DiscordSession.ChannelMessageSend(m.ChannelID, "Joining big poppa")
		}
		return
	}

	// Rest of the commands require a player present in the guild
	player := GetPlayer(guild.ID)
	if player == nil {
		return
	}

	switch strings.ToLower(split[0]) {
	case CmdPrefix + "play", CmdPrefix + "resume":
		player.evtChan <- &PlayerEvtResume{}
		DiscordSession.ChannelMessageSend(m.ChannelID, "Resuuuuuuumed")
	case CmdPrefix + "pause", CmdPrefix + "stop":
		player.evtChan <- &PlayerEvtPause{}
		DiscordSession.ChannelMessageSend(m.ChannelID, "PAuuuuuuused")
	case CmdPrefix + "add":
		if len(split) < 2 {
			err = errors.New("Nothing to add specified")
			break
		}

		what := split[1]
		err = player.QueueUp(what)
		if err == nil {
			DiscordSession.ChannelMessageSend(m.ChannelID, "Did it dad!")
		}
	case CmdPrefix + "next":
		player.evtChan <- &PlayerEvtNext{}
		DiscordSession.ChannelMessageSend(m.ChannelID, "Neeeeext")
	case CmdPrefix + "status":
		status := player.Status()

		itemDuration := "Your moms weight"
		itemName := "Utter silence by moosicman :'("
		if status.Current != nil {
			itemDuration = status.Current.Duration.String()
			itemName = status.Current.Title
		}

		out := fmt.Sprintf("**Player status:**\n**Paused:** %v\n**Title:** %s\n**Position:** %s/%s\n", status.Paused, itemName, status.Position.String(), itemDuration)

		if len(status.Queue) > 0 {
			out += "\n\n**Queue:**\n"
		}

		for k, v := range status.Queue {
			out += fmt.Sprintf("**#%d:** %s - %s (<%s>)\n", k, v.Title, v.Duration.String(), "https://www.youtube.com/watch?v="+v.ID)
		}

		DiscordSession.ChannelMessageSend(m.ChannelID, out)
	case CmdPrefix + "die", CmdPrefix + "kill":
		player.evtChan <- &PlayerEvtKill{}
		DiscordSession.ChannelMessageSend(m.ChannelID, "i am kill D:")
	}

	if err != nil {
		log.Println("Error occured:", err)
		DiscordSession.ChannelMessageSend(m.ChannelID, "Error occured: "+err.Error())
	}
}
