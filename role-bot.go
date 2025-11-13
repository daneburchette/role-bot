package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/fsnotify/fsnotify"
)

type RoleEntry struct {
	Emoji  string `json:"emoji"`
	RoleID string `json:"role_id"`
	Label  string `json:"label"`
}

type ChannelConfig struct {
	ChannelID string      `json:"channel_id"`
	MessageID string      `json:"message_id"`
	Roles     []RoleEntry `json:"roles"`
}

type BotConfig struct {
	BotToken string          `json:"bot_token"`
	Channels []ChannelConfig `json:"channels"`
}

var (
	config           BotConfig
	activeConfigPath string
	logFile          *os.File
)

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to detect home directory: %v", err)
	}
	return filepath.Join(home, ".config", "role-bot", "config.json")
}

func LoadConfig(pathOverride string) {
	path := pathOverride
	if path == "" {
		path = configPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("⚙️  Config not found — generating new template at:", path)
			os.MkdirAll(filepath.Dir(path), 0755)
			config = BotConfig{
				BotToken: "PUT_YOUR_TOKEN_HERE",
				Channels: []ChannelConfig{
					{
						ChannelID: "YOUR_CHANNEL_ID_HERE",
						MessageID: "",
						Roles:     []RoleEntry{{Emoji: "🔥", RoleID: "ROLE_ID_HERE", Label: "Example Role"}},
					},
				},
			}
			SaveConfig(path)
			fmt.Println("✅ Default config created.")
			fmt.Println("Please edit it with your bot token, channel IDs, and roles, then restart.")
			os.Exit(0)
		}
		log.Fatalf("failed to read config: %v", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("failed to parse config JSON: %v", err)
	}
	log.Printf("📄 Config loaded from %s", path)
}

func SaveConfig(path string) {
	if path == "" {
		path = configPath()
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Fatalf("failed to encode config: %v", err)
	}
	os.WriteFile(path, data, 0644)
	log.Printf("💾 Config saved to %s", path)
}

func initLogger(cfgPath string) {
	logDir := filepath.Dir(cfgPath)
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, "logs.txt")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	logFile = f
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.Printf("🪵 Logging initialized at %s", logPath)
}

func parseEmoji(e string) (name, id string) {
	if len(e) > 0 && e[0] == '<' {
		trimmed := e[1 : len(e)-1]
		parts := strings.Split(trimmed, ":")
		if len(parts) == 3 {
			return parts[1], parts[2]
		}
	}
	return e, ""
}

func buildRoleMessage(cfg ChannelConfig) string {
	if len(cfg.Roles) == 0 {
		return "No roles configured yet."
	}
	var b strings.Builder
	b.WriteString("React to assign or remove roles:\n\n")
	for _, r := range cfg.Roles {
		b.WriteString(fmt.Sprintf("%s → %s\n", r.Emoji, r.Label))
	}
	return b.String()
}

func ensureRoleMessage(s *discordgo.Session, ch *ChannelConfig) {
	content := buildRoleMessage(*ch)

	if ch.MessageID == "" {
		msg, err := s.ChannelMessageSend(ch.ChannelID, content)
		if err != nil {
			log.Printf("failed to create role message in %s: %v", ch.ChannelID, err)
			return
		}
		ch.MessageID = msg.ID
		SaveConfig("")
		log.Printf("📨 Created new role message in channel %s", ch.ChannelID)
	} else {
		_, err := s.ChannelMessageEdit(ch.ChannelID, ch.MessageID, content)
		if err != nil {
			log.Printf("failed to edit role message in %s: %v", ch.ChannelID, err)
		} else {
			log.Printf("✏️  Updated role message in channel %s", ch.ChannelID)
		}
	}

	for _, r := range ch.Roles {
		name, id := parseEmoji(r.Emoji)
		var emoji string
		if id != "" {
			emoji = fmt.Sprintf("<:%s:%s>", name, id)
		} else {
			emoji = name
		}
		s.MessageReactionAdd(ch.ChannelID, ch.MessageID, emoji)
	}
}

func onReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	for _, ch := range config.Channels {
		if r.ChannelID != ch.ChannelID || r.MessageID != ch.MessageID {
			continue
		}
		for _, role := range ch.Roles {
			name, id := parseEmoji(role.Emoji)
			if (id != "" && r.Emoji.ID == id) || (id == "" && r.Emoji.Name == name) {
				s.GuildMemberRoleAdd(r.GuildID, r.UserID, role.RoleID)
				log.Printf("✅ Added role %s to user %s", role.Label, r.UserID)
				return
			}
		}
		s.MessageReactionRemove(ch.ChannelID, ch.MessageID, r.Emoji.APIName(), r.UserID)
	}
}

func onReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	for _, ch := range config.Channels {
		if r.ChannelID != ch.ChannelID || r.MessageID != ch.MessageID {
			continue
		}
		for _, role := range ch.Roles {
			name, id := parseEmoji(role.Emoji)
			if (id != "" && r.Emoji.ID == id) || (id == "" && r.Emoji.Name == name) {
				s.GuildMemberRoleRemove(r.GuildID, r.UserID, role.RoleID)
				log.Printf("❌ Removed role %s from user %s", role.Label, r.UserID)
				return
			}
		}
	}
}

func watchConfig(bot *discordgo.Session) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("config watcher error: %v", err)
		return
	}
	defer watcher.Close()

	dir := filepath.Dir(activeConfigPath)
	err = watcher.Add(dir)
	if err != nil {
		log.Printf("failed to watch config dir: %v", err)
		return
	}

	log.Printf("👀 Watching for config changes in %s", dir)
	var lastUpdate time.Time

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Printf("fsnotify event: %+v", event)
			if filepath.Base(event.Name) == filepath.Base(activeConfigPath) &&
				event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				if time.Since(lastUpdate) < 2*time.Second {
					continue
				}
				lastUpdate = time.Now()
				log.Printf("🔄 Detected config change — reloading...")
				LoadConfig(activeConfigPath)
				for i := range config.Channels {
					ensureRoleMessage(bot, &config.Channels[i])
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

func main() {
	configOverride := flag.String("config", "", "Path to config file (optional override)")
	tokenFile := flag.String("token-file", "", "Path to a file containing the bot token (overrides config bot_token)")
	flag.Parse()

	LoadConfig(*configOverride)

	if *configOverride != "" {
		activeConfigPath, _ = filepath.Abs(*configOverride)
	} else {
		activeConfigPath = configPath()
	}

	// initLogger(activeConfigPath)

	if *tokenFile != "" {
		tokenBytes, err := os.ReadFile(*tokenFile)
		if err != nil {
			log.Fatalf("failed reading token file: %v", err)
		}
		config.BotToken = strings.TrimSpace(string(tokenBytes))
	}

	if config.BotToken == "" || config.BotToken == "PUT_YOUR_TOKEN_HERE" {
		log.Fatal("Set your bot_token in the config before running.")
	}

	bot, err := discordgo.New("Bot " + config.BotToken)
	if err != nil {
		log.Fatalf("failed to create Discord session: %v", err)
	}

	bot.AddHandler(onReactionAdd)
	bot.AddHandler(onReactionRemove)

	err = bot.Open()
	if err != nil {
		log.Fatalf("failed to connect to Discord: %v", err)
	}

	for i := range config.Channels {
		ensureRoleMessage(bot, &config.Channels[i])
	}

	go watchConfig(bot)

	fmt.Println("🤖 Bot is now running. Press CTRL-C to exit.")
	select {}
}
