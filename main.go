package main

import (
    "fmt"
    "log"
    "strconv"

    "telegram-order-bot/orders"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotState int

const (
    StateStart BotState = iota
    StateWaitingForProduct
    StateWaitingForAddress
    StateWaitingForPhone
)

type UserSession struct {
    State   BotState
    Product string
    Address string
}

type OrderBot struct {
    bot          *tgbotapi.BotAPI
    orderManager *orders.OrderManager
    sessions     map[int64]*UserSession
    dispatcherID int64 // ID диспетчера для уведомлений
}

func NewOrderBot(token string, dispatcherID int64) (*OrderBot, error) {
    bot, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        return nil, err
    }

    return &OrderBot{
        bot:          bot,
        orderManager: orders.NewOrderManager(),
        sessions:     make(map[int64]*UserSession),
        dispatcherID: dispatcherID,
    }, nil
}

func (b *OrderBot) Start() {
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates := b.bot.GetUpdatesChan(u)

    for update := range updates {
        if update.Message == nil {
            continue
        }

        b.handleMessage(update.Message)
    }
}

func (b *OrderBot) handleMessage(message *tgbotapi.Message) {
    userID := message.Chat.ID

    // Инициализация сессии пользователя
    if b.sessions[userID] == nil {
        b.sessions[userID] = &UserSession{State: StateStart}
    }

    session := b.sessions[userID]

    switch session.State {
    case StateStart:
        b.handleStartState(message, session)
    case StateWaitingForProduct:
        b.handleProductInput(message, session)
    case StateWaitingForAddress:
        b.handleAddressInput(message, session)
    case StateWaitingForPhone:
        b.handlePhoneInput(message, session)
    }
}

func (b *OrderBot) handleStartState(message *tgbotapi.Message, session *UserSession) {
    switch message.Text {
    case "/start":
        b.sendWelcomeMessage(message.Chat.ID)
    case "📋 Прайс":
        b.sendPriceList(message.Chat.ID)
    case "🛒 Сделать заказ":
        session.State = StateWaitingForProduct
        b.sendMessage(message.Chat.ID, 
            "Что вы хотите заказать? Опишите полностью ваш заказ:\n\n" +
            "Пример: 2 балтики, 1 сухарики, 1 чипсы")
    default:
        b.sendWelcomeMessage(message.Chat.ID)
    }
}

func (b *OrderBot) handleProductInput(message *tgbotapi.Message, session *UserSession) {
    session.Product = message.Text
    session.State = StateWaitingForAddress
    
    b.sendMessage(message.Chat.ID, "Введите адрес доставки:")
}

func (b *OrderBot) handleAddressInput(message *tgbotapi.Message, session *UserSession) {
    session.Address = message.Text
    session.State = StateWaitingForPhone
    
    b.sendMessage(message.Chat.ID, "Введите ваш номер телефона для связи:")
}

func (b *OrderBot) handlePhoneInput(message *tgbotapi.Message, session *UserSession) {
    phone := message.Text
    
    // Создаем заказ
    username := message.From.UserName
    if username == "" {
        username = message.From.FirstName
        if message.From.LastName != "" {
            username += " " + message.From.LastName
        }
    }
    
    order := b.orderManager.CreateOrder(
        message.Chat.ID,
        username,
        session.Product,
        session.Address,
        phone,
    )
    
    // Отправляем уведомление диспетчеру
    b.notifyDispatcher(order)
    
    // Подтверждаем пользователю
    b.sendOrderConfirmation(message.Chat.ID, order)
    
    // Сбрасываем сессию
    b.sessions[message.Chat.ID] = &UserSession{State: StateStart}
}

func (b *OrderBot) sendWelcomeMessage(chatID int64) {
    text := `🍕 Добро пожаловать в сервис заказов!

Выберите действие:`
    
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ReplyMarkup = b.getMainKeyboard()
    
    b.bot.Send(msg)
}

func (b *OrderBot) sendPriceList(chatID int64) {
    text := `📋 Наш прайс-лист:

🍕 Закусон:
• Чипсы - 150₽
• Сухарики - 150₽

🥤 Напитки:
• Coca-Cola - 150₽
• Fanta - 150₽
• Вода - 100₽

🍺 Алкоголь:
• Балтика - 150₽
• Эллей - 150₽
• Корона Бочка - 100₽

💵 Минимальный заказ: 500₽
🚚 Доставка: бесплатно от 1000₽`

    b.sendMessage(chatID, text)
}

func (b *OrderBot) sendOrderConfirmation(chatID int64, order orders.Order) {
    text := fmt.Sprintf(`✅ Ваш заказ принят!

Номер заказа: %s
Заказ: %s
Адрес: %s
Телефон: %s

Ваш заказ направлен диспетчеру. С вами свяжутся в ближайшее время для подтверждения.`,
        order.ID, order.Product, order.Address, order.Phone)

    b.sendMessage(chatID, text)
}

func (b *OrderBot) notifyDispatcher(order orders.Order) {
    if b.dispatcherID == 0 {
        return
    }

    text := fmt.Sprintf(`🚨 НОВЫЙ ЗАКАЗ!

Номер: %s
Клиент: @%s
Заказ: %s
Адрес: %s
Телефон: %s
Время: %s`,
        order.ID, order.Username, order.Product, 
        order.Address, order.Phone, order.CreatedAt.Format("15:04 02.01.2006"))

    b.sendMessage(b.dispatcherID, text)
}

func (b *OrderBot) sendMessage(chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    b.bot.Send(msg)
}

func (b *OrderBot) getMainKeyboard() tgbotapi.ReplyKeyboardMarkup {
    return tgbotapi.NewReplyKeyboard(
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("📋 Прайс"),
            tgbotapi.NewKeyboardButton("🛒 Сделать заказ"),
        ),
    )
}

func main() {
    // Получаем токен бота
    botToken := ""
    if botToken == "" {
        log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
    }

    // ID диспетчера
    dispatcherIDStr := "1155607428"
    var dispatcherID int64 = 0
    if dispatcherIDStr != "" {
        var err error
        dispatcherID, err = strconv.ParseInt(dispatcherIDStr, 10, 64)
        if err != nil {
            log.Printf("Invalid dispatcher ID: %v", err)
        } else {
            log.Printf("Dispatcher notifications enabled for chat ID: %d", dispatcherID)
        }
    }

    // Создаем и запускаем бота
    bot, err := NewOrderBot(botToken, dispatcherID)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Authorized on account %s", bot.bot.Self.UserName)
    
    bot.Start()
}