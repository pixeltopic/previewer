package main

import (
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/pixeltopic/mapset"
)

// shouldPreview determines if the message linked from the srcChannelID is "safe" to preview in destChannelID.
func shouldPreview(s *discordgo.Session, destChannelID, srcChannelID string) (bool, error) {

	if destChannelID == "" || srcChannelID == "" {
		return false, errors.New("empty channel ID provided")
	}

	if destChannelID == srcChannelID {
		return true, nil
	}

	srcCh, err := s.State.Channel(srcChannelID)
	if err != nil {
		srcCh, err = s.Channel(srcChannelID)
		if err != nil {
			return false, err
		}
		_ = s.State.ChannelAdd(srcCh)
	}

	destCh, err := s.State.Channel(destChannelID)
	if err != nil {
		destCh, err = s.Channel(destChannelID)
		if err != nil {
			return false, err
		}
		_ = s.State.ChannelAdd(destCh)
	}

	if srcCh.NSFW && !destCh.NSFW {
		return false, nil
	}

	var (
		deniedSrcRoles  = mapset.NewThreadUnsafeSet()
		deniedDestRoles = mapset.NewThreadUnsafeSet()
	)

	for _, perm := range srcCh.PermissionOverwrites {
		shouldDeny := (perm.Deny|discordgo.PermissionReadMessageHistory == perm.Deny) || (perm.Deny|discordgo.PermissionReadMessages == perm.Deny)
		if shouldDeny && perm.Type == "role" { // ignore "member" type
			deniedSrcRoles.Add(perm.ID)
		}
	}

	for _, perm := range destCh.PermissionOverwrites {
		shouldDeny := (perm.Deny|discordgo.PermissionReadMessageHistory == perm.Deny) || (perm.Deny|discordgo.PermissionReadMessages == perm.Deny)
		if shouldDeny && perm.Type == "role" { // ignore "member" type
			deniedDestRoles.Add(perm.ID)
		}
	}

	return deniedSrcRoles.Equal(deniedDestRoles) || deniedSrcRoles.IsProperSubset(deniedDestRoles), nil
}
