package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// Limits the amount of previews that can be sent for a single message.
const maxEmbedPerMsg = 3

var (
	// Token stores the Discord API token
	Token string
	reg   = regexp.MustCompile(`https://(discordapp\.com|discord\.com)/channels/(\d+)/(\d+)/(\d+)`)
)

type linkMeta struct {
	link      string
	guildID   string
	channelID string
	messageID string
}

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

// comesFromDM returns true if a message comes from a DM channel
func comesFromDM(s *discordgo.Session, m *discordgo.Message) bool {

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		if channel, err = s.Channel(m.ChannelID); err != nil {
			return len(m.GuildID) == 0 // final fallback to check if DM
		}
	}

	return channel.Type == discordgo.ChannelTypeDM
}

func extractLinkMeta(s string) ([]linkMeta, error) {
	matches := reg.FindAllStringSubmatch(s, maxEmbedPerMsg)

	var lm []linkMeta

	for _, m := range matches {
		if len(m) < 5 {
			return nil, errors.New("invalid submatch found")
		}

		lm = append(lm, linkMeta{
			link:      m[0],
			guildID:   m[2],
			channelID: m[3],
			messageID: m[4],
		})
	}

	return lm, nil
}

func preview(ses *discordgo.Session, msg *discordgo.MessageCreate) {

	if comesFromDM(ses, msg.Message) {
		return
	}

	extracted, err := extractLinkMeta(msg.Content)
	if err != nil {
		log.Printf("Command failed: %+v\n", err)
		return
	}
	var metaToPreview []linkMeta

	for _, m := range extracted {
		if m.guildID == msg.GuildID {
			metaToPreview = append(metaToPreview, m)
		}
	}

	if len(metaToPreview) == 0 {
		return
	}

	for i, meta := range metaToPreview {
		if i == maxEmbedPerMsg {
			return
		}
		msgs, err := ses.ChannelMessages(meta.channelID, 1, "", "", meta.messageID)
		if err != nil {
			log.Printf("Command failed: %+v\n", err)
			return
		}

		if len(msgs) == 0 {
			continue
		}

		m := msgs[0]

		if m.ID == meta.messageID {

			content := m.Content
			if c, err := m.ContentWithMoreMentionsReplaced(ses); err == nil {
				content = c
			}

			if 128 <= len(content) {
				content = strings.TrimSpace(content[:128]) + "..."
			}

			var (
				imgs            []*discordgo.MessageAttachment
				attachmentNames []string
			)

			for _, a := range m.Attachments {
				if a.Width > 0 && a.Height > 0 {
					imgs = append(imgs, a)
				} else {
					attachmentNames = append(attachmentNames, a.Filename)
				}
			}

			t, _ := m.Timestamp.Parse()

			var chanName string
			ch, err := ses.State.Channel(m.ChannelID)
			if err != nil {
				chanName = "#Unknown Channel"
			} else {
				chanName = "#" + ch.Name
			}

			embed := &discordgo.MessageEmbed{
				URL:         meta.link,
				Title:       "Linked Message Preview",
				Description: content,
				Timestamp:   t.Format("2006-01-02 15:04:05"),
				Color:       7188182,
				Footer: &discordgo.MessageEmbedFooter{
					IconURL: m.Author.AvatarURL("128"),
					Text:    fmt.Sprintf("Message sent by %s#%s in %s", m.Author.Username, m.Author.Discriminator, chanName),
				},
			}

			if len(imgs) != 0 {
				embed.Image = &discordgo.MessageEmbedImage{
					URL:      imgs[0].URL,
					ProxyURL: imgs[0].ProxyURL,
					Width:    imgs[0].Width,
					Height:   imgs[0].Height,
				}
			}

			if len(attachmentNames) != 0 {
				embed.Fields = []*discordgo.MessageEmbedField{
					{
						Name:  "Attached Files",
						Value: strings.Join(attachmentNames, ", "),
					},
				}
			}

			allow, err := shouldPreview(ses, msg.ChannelID, m.ChannelID)
			if err != nil {
				log.Printf("Command failed: %+v\n", err)
				return
			}

			if allow {
				_, _ = ses.ChannelMessageSendEmbed(msg.ChannelID, embed)
			}
		}
	}
}

func main() {
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(preview)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	_ = dg.Close()
}
