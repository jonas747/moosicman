package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	Version = "0.0.1"
)

// Variables used for command line parameters
var (
	Token        string
	CmdPrefix    string
	PlayersPath  string
	MaxQueueSize int

	DiscordSession *discordgo.Session

	ErrNoPlayer = errors.New("No player in this server D:")
)

func init() {
	flag.StringVar(&Token, "t", "", "Account Token")
	flag.StringVar(&CmdPrefix, "p", ">", "Command prefix")
	flag.StringVar(&PlayersPath, "pl", "players.json", "Where the players should be stored")
	flag.IntVar(&MaxQueueSize, "mq", 100, "Max queue size")
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

	ListenStopSignal()
}

func handleReady(s *discordgo.Session, m *discordgo.Ready) {
	log.Println("Ready received! Fire away with the commands b0ss")
	log.Println("If this is a bot account people acn invite it with the following link:")
	log.Println("https://discordapp.com/oauth2/authorize?client_id=CLIENT_ID_HERE&scope=bot")
	log.Println("Replace 'CLIENT_ID_HERE' with the bot's client id")

	err := LoadPlayersFromDisk(PlayersPath)
	if err != nil {
		log.Println("Error loading players", err)
	}
}

func handleGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	log.Printf("Joined guild %s (%s)\n", g.Name, g.ID)
}

func handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	split := strings.SplitN(m.Content, " ", 2)

	if strings.Index(split[0], CmdPrefix) != 0 {
		return
	}

	cmd := strings.Replace(strings.ToLower(split[0]), CmdPrefix, "", 1)

	if cmd == "help" {
		DiscordSession.ChannelMessageSend(m.ChannelID, GenCmdHelp())
		return
	}

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

	if cmd == "join" {
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

	switch cmd {
	case "die", "kill", "leave":
		// Kills the player violently
		player.evtChan <- &PlayerEvtKill{}
		DiscordSession.ChannelMessageSend(m.ChannelID, "i am kill D:")

	////////////////////
	// PLAYBACK CONTROL
	////////////////////
	case "play", "resume":
		// Resumes/plays
		player.evtChan <- &PlayerEvtResume{}
		DiscordSession.ChannelMessageSend(m.ChannelID, "Resuuuuuuumed")
	case "pause", "stop":
		// Pauses the playback
		player.evtChan <- &PlayerEvtPause{}
		DiscordSession.ChannelMessageSend(m.ChannelID, "PAuuuuuuused")
	case "add":
		// Adds another element to the queue
		if len(split) < 2 {
			err = errors.New("Nothing to add specified")
			break
		}

		what := split[1]
		err = player.QueueUp(what)
		if err == nil {
			DiscordSession.ChannelMessageSend(m.ChannelID, "Did it dad!")
		}
	case "next", "skip":
		// Skips to the next one
		player.evtChan <- &PlayerEvtNext{Index: -1}
		DiscordSession.ChannelMessageSend(m.ChannelID, "Neeeeext")
	case "randnext", "rnext":
		// Skips to a random item in the playlist
		player.evtChan <- &PlayerEvtNext{Index: -1, Random: true}
		DiscordSession.ChannelMessageSend(m.ChannelID, "RAAaandom next :o")
	case "goto", "item", "skipto":
		// Skips to a specific item in the playlist
		if len(split) < 2 {
			err = errors.New("No item index specified you dillweed")
			break
		}

		index, err := strconv.Atoi(split[1])
		if err != nil {
			break
		}

		if index < 0 {
			err = errors.New("No. >:O")
			break
		}

		player.evtChan <- &PlayerEvtNext{Index: index}
		DiscordSession.ChannelMessageSend(m.ChannelID, "Playing a specific one eh?")

	//////////////////
	// UTILIITIES
	//////////////////
	case "status":
		// Prints player status
		status := player.Status()

		itemDuration := "Your moms weight"
		itemName := "Utter silence by moosicman :'("
		if status.Current != nil {
			itemDuration = status.Current.Duration.String()
			itemName = status.Current.Title
		}

		out := fmt.Sprintf("**Player status:**\n**Paused:** %v\n**Title:** %s\n**Position:** %s/%s\n**Shuffle:** %v\n**Persist:** %v\n", status.Paused, itemName, status.Position.String(), itemDuration, status.Shuffle, status.Persist)

		if len(status.Queue) > 0 {
			out += "\n\n**Queue:**\n"
		}

		for k, v := range status.Queue {
			out += fmt.Sprintf("**#%d:** %s - %s (<%s>)\n", k, v.Title, v.Duration.String(), "https://www.youtube.com/watch?v="+v.ID)
		}

		DiscordSession.ChannelMessageSend(m.ChannelID, out)

	case "persist":
		// Toggle persiting the queue (items stays in the list after)
		persisting := player.TogglePersist()
		DiscordSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Persist: %v", persisting))
	case "shuffle":
		// Enters shuffle mode where the next item is picked randomly
		shuffle := player.ToggleShuffle()
		DiscordSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Shuffle: %v", shuffle))
	case "remove":
		// Removes an element in the playlist
		index := 0
		index, err = strconv.Atoi(split[1])
		if err != nil {
			break
		}

		if index < 0 {
			err = errors.New("No. >:O")
			break
		}
		player.evtChan <- &PlayerEvtRemove{Index: index}
	}

	if err != nil {
		log.Println("Error occured:", err)
		DiscordSession.ChannelMessageSend(m.ChannelID, "Error occured: "+err.Error())
	}
}

func GenCmdHelp() string {
	out := strings.Replace(cmdHelp, "{{ver}}", Version, -1)
	out = strings.Replace(out, "{{pref}}", CmdPrefix, -1)
	return out
}

const cmdHelp = `**Moosicman version {{ver}} by a very beautiful man**
` + "```" + `
Important:
{{pref}}help       : Shows this menu
{{pref}}join       : Joins a voice channel (needs to be used before any other commands)
{{pref}}kill/leave : Kills the player and makes the bo leave the voice channel (queue will be deleted aswell)

Playback Control:
{{pref}}add <youtubelink>      : Adds the youtube link to the queue
{{pref}}remove <item index>    : Removes an element in the queue
{{pref}}stop/pause             : Pauses the bot
{{pref}}play/resume            : Resumes playing
{{pref}}next/skip              : Goes to the next item in the queue
{{pref}}randnext/rnext         : Goes to a random element in the queue
{{pref}}goto/item <item index> : Plays the specified item in the queue

Utilities:
{{pref}}status    : Shows the bot's status
{{pref}}persist   : Toggles queue persist mode, After items are played they wont get removed
{{pref}}shuffle   : Toggles shuffle mode
` + "```" + `
**If you have any issues then throw your computer out the window**
`
