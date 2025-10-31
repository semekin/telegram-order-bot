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
    text := `📋В наличии большой ассортимент. По элитному алкоголю звоните, все расскажем
——————————
Водка:
5 озер 0,5 1000₽, 1 литр 2000₽
Подледка 0,5 1000₽, 0,7 1500₽, 1 литр 2000₽
Тундра 0,5 1200₽, 1 литр 2400₽
Хаски 0,5 1200₽, 0,7 1500₽, 1 литр 2400₽
АМГ угольная 0,5 1200₽, литр 2400₽
Дархан 0,5 2000₽
Финляндия 0,5 2500₽, 0,7 3500₽, 1 литр 5000₽
Абсолют 0,5 3000₽, 1 литр 5000₽
Белуга 0,5 2400₽, 1 литр 4000₽
Онегин 07 3000₽
——————————
Пиво:
0.33: корона экстра 400₽ (доставка от 6 шт.)
0.5: бад 200₽ (доставка от 6 шт.)
0.5: бад лайт 250₽ (доставка от 6 шт.)❗️снова в наличии 😍
0.5: жигулевское(1978) 200₽ (доставка от 6 шт.)
0.5: козел светлый 250₽ (доставка от 6 шт.)
0.5: козел темный 250₽ (доставка от 6 шт.)
0.5: окоме 250₽ (доставка от 6 шт.)
0.5: мистер лис 250₽ (доставка от 6 шт.)
0.5: харбин 400₽ (доставка от 6 шт.)
1,5: чешское (Бочкарев) 400₽ (доставка от 3 шт.)
1,5: жигулевское (1978) 400₽ (доставка от 3 шт.)
——————————
Вино, шампанское, вермут, аперитив:
Вино Испания: Эспада Дорада (красное п/сл, белое п/сл, красное сух, белое сух) 1400₽
Вино Грузия: Киндзмараули (кр/псл), Мукузани (кр/сух), Цинандали (бел/сух), Алазанская долина (бел/псл) 1600₽
Вермут: Мартини 0,5 2500₽, 1 литр 3800₽
Санта Стефано (это вермут!!! Не игристое вино) 0,75 1500₽, 1 литр 2000₽
Шампанское: Российское (брют, п/сл) 1 шт-1000₽
Абрау Дюрсо (брют, п/сл) 1 шт-1300₽
Аперитив: Апероль (0,7) - 4000₽
——————————
Коньяк, бренди:
Киновский (0,5) 3* 1800₽, 5* 1900₽
Золотая Якутия (0,5) 3* 1800₽, 5* 1800₽
Монте Чоко (0,5) 5* 2000₽
Армянский (матовая бутылка) (0,5) 3*, 5* 2000₽
Курвуазье VS (0,5) 10000₽
Курвуазье VS (0,7) 12000₽
Курвуазье VSOP (0,7) 14000₽
Мартель VS (0,7) 13000₽
Торрес 5 (0,5) - 3500₽
Торрес 5 (0,7) - 5000₽
Торрес 10 (0,5) - 4000₽
——————————
Виски:
Fox&Dogs (0,5)-2000₽
Балантайз (0,5)-4000₽, (0,7)-5000₽
Джим Бим (0,5)-5000₽, (0,7) (обычный) - 6000₽, (0,7) (яблочный, медовый)-6500₽
Джек Дэниелс (0,5)-5000₽, (0,7) (обычный, медовый)-6000₽
Джемесон (0,5)-5000₽, (0,7)-6000₽
Вильям Лоусон (0,5)-3000₽, (0,7) - 4000₽
Ред Лейбл (0,7) - 5000₽
Пропер Твелв (0,7) - 6000₽
Чивас Ригал (0,7) - 10000₽
Чивас Ригал 1 л - 12000₽
——————————
Текила, ром, абсент, самбука, джин:
Ольмека Голд 07 - 8000₽
Ольмека Сильвер 07 - 6000₽
Сауза Сильвер 07 - 8000₽
Бакарди Карта Бланка 07 - 6000₽
Бакарди Карта Негра 07 - 6000₽
Капитан Морган Блэк 07 - 4000₽
Джин Барристер 07 - 2500₽
Джин Барристер пинк 07 - 2500₽
Джин Гордонс 07 - 5000₽

——————————
Ликер, бальзам:
Ягермастер (0,7) - 5000₽
Бейлиз (0,7) - 5000₽
——————————    `

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
    botToken := "8409546502:AAHMu4vLc03J-pTXyzcbyvP9TikCVTorllc"
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
