go
package main

import (
    "fmt"
    "log"
    "os"
    "strconv"
    "strings"

    "telegram-order-bot/orders"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotState int

const (
    StateStart BotState = iota
    StateWaitingForProduct
    StateWaitingForQuantity
    StateWaitingForAddress
    StateWaitingForPhone
)

type UserSession struct {
    State    BotState
    Product  string
    Quantity int
    Address  string
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
    text := message.Text

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
    case StateWaitingForQuantity:
        b.handleQuantityInput(message, session)
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
            "Что вы хотите заказать? Опишите продукт:\n\n" +
            "• Пицца Маргарита - 550₽\n" +
            "• Пицца Пепперони - 650₽\n" +
            "• Бургер Классический - 350₽\n" +
            "• Салат Цезарь - 300₽\n" +
            "• Напиток Coca-Cola - 150₽")
    default:
        b.sendWelcomeMessage(message.Chat.ID)
    }
}

func (b *OrderBot) handleProductInput(message *tgbotapi.Message, session *UserSession) {
    session.Product = message.Text
    session.State = StateWaitingForQuantity
    
    b.sendMessage(message.Chat.ID, "Введите количество:")
}

func (b *OrderBot) handleQuantityInput(message *tgbotapi.Message, session *UserSession) {
    quantity, err := strconv.Atoi(message.Text)
    if err != nil || quantity <= 0 {
        b.sendMessage(message.Chat.ID, "Пожалуйста, введите корректное количество (число больше 0):")
        return
    }

    session.Quantity = quantity
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
        username = message.From.FirstName + " " + message.From.LastName
    }
    
    order := b.orderManager.CreateOrder(
        message.Chat.ID,
        username,
        session.Product,
        session.Address,
        phone,
        session.Quantity,
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

🍕 Пиццы:
• Маргарита - 550₽
• Пепперони - 650₽
• Гавайская - 600₽

🍔 Бургеры:
• Классический - 350₽
• Чизбургер - 400₽
• Двойной - 500₽

🥗 Салаты:
• Цезарь - 300₽
• Греческий - 280₽

🥤 Напитки:
• Coca-Cola - 150₽
• Fanta - 150₽
• Вода - 100₽

💵 Минимальный заказ: 500₽
🚚 Доставка: бесплатно от 1000₽`

    b.sendMessage(chatID, text)
}

func (b *OrderBot) sendOrderConfirmation(chatID int64, order orders.Order) {
    text := fmt.Sprintf(`✅ Ваш заказ принят!

Номер заказа: %s
Продукт: %s
Количество: %d
Адрес: %s
Телефон: %s

Ваш заказ направлен диспетчеру. С вами свяжутся в ближайшее время для подтверждения.`,
        order.ID, order.Product, order.Quantity, order.Address, order.Phone)

    b.sendMessage(chatID, text)
}

func (b *OrderBot) notifyDispatcher(order orders.Order) {
    if b.dispatcherID == 0 {
        return
    }

    text := fmt.Sprintf(`🚨 НОВЫЙ ЗАКАЗ!

Номер: %s
Клиент: @%s
Товар: %s
Количество: %d
Адрес: %s
Телефон: %s
Время: %s`,
        order.ID, order.Username, order.Product, order.Quantity, 
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
    // Получаем токен бота из переменных окружения
    botToken := os.Getenv("8409546502:AAHMu4vLc03J-pTXyzcbyvP9TikCVTorllc")
    if botToken == "" {
        log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
    }

    // ID диспетчера (можно получить через @userinfobot)
    dispatcherIDStr := os.Getenv("7728044697")
    var dispatcherID int64 = 0
    if dispatcherIDStr != "" {
        var err error
        dispatcherID, err = strconv.ParseInt(dispatcherIDStr, 10, 64)
        if err != nil {
            log.Printf("Invalid dispatcher ID: %v", err)
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
