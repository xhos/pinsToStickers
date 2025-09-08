package bot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/sirupsen/logrus"
	"github.com/xhos/itanoru/internal/database"
	"github.com/xhos/itanoru/internal/sync"
)

type Handler struct {
	db     *database.DB
	log    *logrus.Logger
	syncer *sync.Engine
}

func NewHandler(db *database.DB, logger *logrus.Logger, syncer *sync.Engine) *Handler {
	return &Handler{
		db:     db,
		log:    logger,
		syncer: syncer,
	}
}

func (h *Handler) HandleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	if msg.Text == "" {
		return
	}

	user, err := h.db.GetOrCreateUser(msg.From.ID, msg.From.Username)
	if err != nil {
		h.log.WithError(err).Error("Failed to get or create user")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to process user information")
		return
	}

	h.log.WithFields(logrus.Fields{
		"user_id":     user.TelegramID,
		"username":    user.Username,
		"message":     msg.Text,
	}).Info("Processing command")

	parts := strings.Fields(msg.Text)
	if len(parts) == 0 {
		return
	}

	command := strings.ToLower(parts[0])
	switch command {
	case "/start":
		h.handleStart(ctx, b, msg, user)
	case "/newpack":
		h.handleNewPack(ctx, b, msg, user, parts[1:])
	case "/sync":
		h.handleSync(ctx, b, msg, user, parts[1:])
	case "/syncall":
		h.handleSyncAll(ctx, b, msg, user)
	case "/listpacks":
		h.handleListPacks(ctx, b, msg, user)
	case "/deletepack":
		h.handleDeletePack(ctx, b, msg, user, parts[1:])
	case "/packinfo":
		h.handlePackInfo(ctx, b, msg, user, parts[1:])
	case "/help":
		h.handleHelp(ctx, b, msg)
	default:
		h.handleUnknown(ctx, b, msg)
	}
}

func (h *Handler) handleStart(ctx context.Context, b *bot.Bot, msg *models.Message, user *database.User) {
	welcome := `🎨 Welcome to Itanoru - Pinterest to Telegram Sticker Bot!

This bot helps you synchronize Pinterest boards with Telegram sticker packs.

Available commands:
• /newpack <pinterest_board_url> - Create a new sticker pack from Pinterest board
• /listpacks - Show all your sticker packs
• /sync <pack_name> - Manually sync a specific pack
• /syncall - Sync all your packs
• /packinfo <pack_name> - Show pack details
• /deletepack <pack_name> - Delete pack management
• /help - Show this help message

To get started, use /newpack with a Pinterest board URL!`

	h.sendMessage(ctx, b, msg.Chat.ID, welcome)
}

func (h *Handler) handleNewPack(ctx context.Context, b *bot.Bot, msg *models.Message, user *database.User, args []string) {
	if len(args) == 0 {
		h.sendMessage(ctx, b, msg.Chat.ID, "❌ Please provide a Pinterest board URL.\nUsage: /newpack <pinterest_board_url>")
		return
	}

	boardURL := args[0]
	if !h.isValidPinterestURL(boardURL) {
		h.sendMessage(ctx, b, msg.Chat.ID, "❌ Invalid Pinterest board URL. Please provide a valid Pinterest board URL.")
		return
	}

	packName := h.generatePackName(user.Username, boardURL)
	packTitle := fmt.Sprintf("Pinterest Board Stickers")

	existingPack, err := h.db.GetPackByName(user.ID, packName)
	if err != nil {
		h.log.WithError(err).Error("Failed to check existing pack")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to check existing packs")
		return
	}

	if existingPack != nil {
		h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("⚠️ You already have a sticker pack for this Pinterest board: %s", existingPack.TelegramPackName))
		return
	}

	pack, err := h.db.CreateStickerPack(user.ID, packName, boardURL, "", packTitle)
	if err != nil {
		h.log.WithError(err).Error("Failed to create sticker pack")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to create sticker pack")
		return
	}

	h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("✅ Created new sticker pack: %s\n🔄 Starting initial sync...", packName))

	go func() {
		if err := h.syncer.SyncPack(pack.ID); err != nil {
			h.log.WithError(err).Error("Failed to sync pack")
			h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("❌ Failed to sync pack %s: %v", packName, err))
		} else {
			h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("🎉 Successfully synced pack: %s\nYou can now use it in Telegram!", packName))
		}
	}()
}

func (h *Handler) handleSync(ctx context.Context, b *bot.Bot, msg *models.Message, user *database.User, args []string) {
	if len(args) == 0 {
		h.sendMessage(ctx, b, msg.Chat.ID, "❌ Please provide a pack name.\nUsage: /sync <pack_name>")
		return
	}

	packName := args[0]
	pack, err := h.db.GetPackByName(user.ID, packName)
	if err != nil {
		h.log.WithError(err).Error("Failed to get pack")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to get pack information")
		return
	}

	if pack == nil {
		h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("❌ Pack '%s' not found. Use /listpacks to see your packs.", packName))
		return
	}

	h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("🔄 Starting sync for pack: %s", packName))

	go func() {
		if err := h.syncer.SyncPack(pack.ID); err != nil {
			h.log.WithError(err).Error("Failed to sync pack")
			h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("❌ Failed to sync pack %s: %v", packName, err))
		} else {
			h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("✅ Successfully synced pack: %s", packName))
		}
	}()
}

func (h *Handler) handleSyncAll(ctx context.Context, b *bot.Bot, msg *models.Message, user *database.User) {
	packs, err := h.db.GetUserPacks(user.ID)
	if err != nil {
		h.log.WithError(err).Error("Failed to get user packs")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to get your packs")
		return
	}

	if len(packs) == 0 {
		h.sendMessage(ctx, b, msg.Chat.ID, "📭 You don't have any sticker packs yet. Use /newpack to create one!")
		return
	}

	h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("🔄 Starting sync for all %d packs...", len(packs)))

	go func() {
		success := 0
		failed := 0
		
		for _, pack := range packs {
			if err := h.syncer.SyncPack(pack.ID); err != nil {
				h.log.WithError(err).WithField("pack_id", pack.ID).Error("Failed to sync pack")
				failed++
			} else {
				success++
			}
		}

		result := fmt.Sprintf("✅ Sync completed!\n📊 Success: %d\n❌ Failed: %d", success, failed)
		h.sendMessage(ctx, b, msg.Chat.ID, result)
	}()
}

func (h *Handler) handleListPacks(ctx context.Context, b *bot.Bot, msg *models.Message, user *database.User) {
	packs, err := h.db.GetUserPacks(user.ID)
	if err != nil {
		h.log.WithError(err).Error("Failed to get user packs")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to get your packs")
		return
	}

	if len(packs) == 0 {
		h.sendMessage(ctx, b, msg.Chat.ID, "📭 You don't have any sticker packs yet. Use /newpack to create one!")
		return
	}

	var response strings.Builder
	response.WriteString("📦 Your Sticker Packs:\n\n")

	for i, pack := range packs {
		status := "❓"
		switch pack.SyncStatus {
		case "completed":
			status = "✅"
		case "syncing":
			status = "🔄"
		case "error":
			status = "❌"
		case "pending":
			status = "⏳"
		}

		response.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, status, pack.TelegramPackName))
		response.WriteString(fmt.Sprintf("   📍 Board: %s\n", pack.BoardName))
		response.WriteString(fmt.Sprintf("   🎯 Stickers: %d\n", pack.StickerCount))
		
		if pack.LastSyncAt != nil {
			response.WriteString(fmt.Sprintf("   🕒 Last sync: %s\n", pack.LastSyncAt.Format("2006-01-02 15:04")))
		}
		response.WriteString("\n")
	}

	h.sendMessage(ctx, b, msg.Chat.ID, response.String())
}

func (h *Handler) handleDeletePack(ctx context.Context, b *bot.Bot, msg *models.Message, user *database.User, args []string) {
	if len(args) == 0 {
		h.sendMessage(ctx, b, msg.Chat.ID, "❌ Please provide a pack name.\nUsage: /deletepack <pack_name>")
		return
	}

	packName := args[0]
	pack, err := h.db.GetPackByName(user.ID, packName)
	if err != nil {
		h.log.WithError(err).Error("Failed to get pack")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to get pack information")
		return
	}

	if pack == nil {
		h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("❌ Pack '%s' not found.", packName))
		return
	}

	h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("⚠️ This will remove pack management for '%s'.\nThe Telegram sticker pack will remain, but won't be synced anymore.\n\nThis action cannot be undone.", packName))
}

func (h *Handler) handlePackInfo(ctx context.Context, b *bot.Bot, msg *models.Message, user *database.User, args []string) {
	if len(args) == 0 {
		h.sendMessage(ctx, b, msg.Chat.ID, "❌ Please provide a pack name.\nUsage: /packinfo <pack_name>")
		return
	}

	packName := args[0]
	pack, err := h.db.GetPackByName(user.ID, packName)
	if err != nil {
		h.log.WithError(err).Error("Failed to get pack")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to get pack information")
		return
	}

	if pack == nil {
		h.sendMessage(ctx, b, msg.Chat.ID, fmt.Sprintf("❌ Pack '%s' not found.", packName))
		return
	}

	stickers, err := h.db.GetPackStickers(pack.ID)
	if err != nil {
		h.log.WithError(err).Error("Failed to get pack stickers")
		h.sendError(ctx, b, msg.Chat.ID, "Failed to get sticker information")
		return
	}

	status := "❓"
	switch pack.SyncStatus {
	case "completed":
		status = "✅ Synced"
	case "syncing":
		status = "🔄 Syncing"
	case "error":
		status = "❌ Error"
	case "pending":
		status = "⏳ Pending"
	}

	info := fmt.Sprintf(`📦 Pack Information

📝 Name: %s
📍 Pinterest Board: %s
🎯 Sticker Count: %d
📊 Status: %s
🕒 Created: %s`,
		pack.TelegramPackName,
		pack.PinterestBoardURL,
		len(stickers),
		status,
		pack.CreatedAt.Format("2006-01-02 15:04"))

	if pack.LastSyncAt != nil {
		info += fmt.Sprintf("\n🔄 Last Sync: %s", pack.LastSyncAt.Format("2006-01-02 15:04"))
	}

	if pack.ErrorMessage != "" {
		info += fmt.Sprintf("\n⚠️ Error: %s", pack.ErrorMessage)
	}

	h.sendMessage(ctx, b, msg.Chat.ID, info)
}

func (h *Handler) handleHelp(ctx context.Context, b *bot.Bot, msg *models.Message) {
	help := `🎨 Itanoru - Pinterest to Telegram Sticker Bot

Available commands:
• /start - Show welcome message
• /newpack <pinterest_board_url> - Create new sticker pack from Pinterest board
• /listpacks - Show all your sticker packs
• /sync <pack_name> - Manually sync a specific pack
• /syncall - Sync all your packs
• /packinfo <pack_name> - Show detailed pack information
• /deletepack <pack_name> - Remove pack management
• /help - Show this help message

📌 Tips:
• Sticker packs sync automatically every hour
• Maximum 120 stickers per pack (Telegram limit)
• Images are automatically converted to WebP format
• Pack names are generated automatically from your username

🔗 Pinterest Board URL format:
https://pinterest.com/username/board-name/`

	h.sendMessage(ctx, b, msg.Chat.ID, help)
}

func (h *Handler) handleUnknown(ctx context.Context, b *bot.Bot, msg *models.Message) {
	h.sendMessage(ctx, b, msg.Chat.ID, "❓ Unknown command. Use /help to see available commands.")
}

func (h *Handler) sendMessage(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		h.log.WithError(err).Error("Failed to send message")
	}
}

func (h *Handler) sendError(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	h.sendMessage(ctx, b, chatID, "❌ "+text)
}

func (h *Handler) isValidPinterestURL(url string) bool {
	pinterestPattern := regexp.MustCompile(`^https?://(?:www\.)?pinterest\.com/[^/]+/[^/]+/?$`)
	return pinterestPattern.MatchString(url)
}

func (h *Handler) generatePackName(username, boardURL string) string {
	urlParts := strings.Split(strings.TrimSuffix(boardURL, "/"), "/")
	if len(urlParts) >= 2 {
		boardName := urlParts[len(urlParts)-1]
		cleanBoard := regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(boardName, "_")
		return fmt.Sprintf("%s_%s_by_itanoru_bot", username, cleanBoard)
	}
	
	return fmt.Sprintf("%s_board_by_itanoru_bot", username)
}