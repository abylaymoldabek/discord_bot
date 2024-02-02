package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/abylaymoldabek/discord_bot/log"

	"github.com/bregydoc/gtranslate"
	owm "github.com/briandowns/openweathermap"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	trapInProgress bool
	trapWord       string
	hint           string
)

func init() {
	// Loading environment variables from a .env file
	err := godotenv.Load()
	if err != nil {
		log.ErrorLogger.Fatal("Error loading .env file")
	}
}

func main() {
	discord, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN")) // replace token
	if err != nil {
		log.ErrorLogger.Fatal("Error creating Discord session,", err)
		return
	}

	discord.Identify.Intents = discordgo.IntentsAll // identify intenits

	discord.AddHandler(messageCreate) // add event handler

	err = discord.Open()
	if err != nil {

		log.ErrorLogger.Fatal("Error opening Discord session,", err)
		return
	}
	defer discord.Close()

	log.InfoLogger.Println("Bot is now running. Press CTRL+C to exit.")
	select {}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) { // create message
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Парсинг команд
	if strings.HasPrefix(m.Content, "!") {
		handleCommand(s, m) // handle command
	}
}

func handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	command := strings.Fields(m.Content)[0]

	switch command {
	case "!hello":
		s.ChannelMessageSend(m.ChannelID, "World")
	case "!weather":
		go handleWeatherCommand(s, m)
	case "!translate":
		go handleTranslateCommand(s, m)
	case "!trap":
		go handleTrapCommand(s, m)
	case "!help":
		sendHelpMessage(s, m)
	}
}

func sendHelpMessage(s *discordgo.Session, m *discordgo.MessageCreate) { // func for help messages for user
	helpMessage := "Доступные команды:\n" +
		"!hello - приветствие\n" +
		"!weather [город] - получение информации о погоде\n" +
		"!translate [from] [to] [текст] - перевод текста\n" +
		"!trap - начать игру 'Ловушка слов'\n" +
		"!help - отображение этого сообщения с командами"
	s.ChannelMessageSend(m.ChannelID, helpMessage)
}

func handleWeatherCommand(s *discordgo.Session, m *discordgo.MessageCreate) { // function for weather
	// Replace "YOUR_OPENWEATHERMAP_API_KEY" with your OpenWeatherMap API key
	// Example: !weather Almaty
	apiKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	args := strings.Fields(m.Content[len("!weather"):])
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "Недостаточно аргументов. Пример: `!weather Almaty`")
		return
	}
	location := args[0]
	w, err := owm.NewCurrent("C", "EN", apiKey)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Error creating OpenWeatherMap client.")
		log.ErrorLogger.Fatal("Error creating OpenWeatherMap client:", err)
		return
	}

	err = w.CurrentByName(location)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Error fetching weather data for the specified location.")
		log.ErrorLogger.Fatal("Error fetching weather data for the specified location:", err)
		return
	}
	if w.Name == "Nur-Sultan" || w.Name == "Нур-Султан" {
		response := fmt.Sprintf("Weather in %s: %s, %.1f°C", "Astana", w.Weather[0].Description, w.Main.Temp)
		s.ChannelMessageSend(m.ChannelID, response)
		return
	}
	response := fmt.Sprintf("Weather in %s: %s, %.1f°C", w.Name, w.Weather[0].Description, w.Main.Temp)
	s.ChannelMessageSend(m.ChannelID, response)
}

func handleTranslateCommand(s *discordgo.Session, m *discordgo.MessageCreate) { // function for translate
	// Example: !translate en ru hello
	args := strings.Fields(m.Content[len("!translate"):])
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Недостаточно аргументов. Пример: `!translate ru en Привет, как дела?`")
		return
	}
	fromLanguage := args[0]
	targetLanguage := args[1]
	textToTranslate := strings.Join(args[2:], " ")
	params := gtranslate.TranslationParams{
		From: fromLanguage,
		To:   targetLanguage,
	}

	translatedText, err := gtranslate.TranslateWithParams(textToTranslate, params)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Error translating text.")
		log.ErrorLogger.Fatal("Error translating text:", err)
		return
	}

	response := fmt.Sprintf("Translated text (%s): %s", targetLanguage, translatedText)
	s.ChannelMessageSend(m.ChannelID, response)
}

func handleTrapCommand(s *discordgo.Session, m *discordgo.MessageCreate) { // function for trap game
	if trapInProgress {
		s.ChannelMessageSend(m.ChannelID, "Игра 'Ловушка слов' уже идет. Завершите текущую игру, прежде чем начать новую.")
		return
	}

	trapWord, hint = generateTrapWord()

	displayWord := hideRandomLetters(trapWord)

	trapInProgress = true
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Новая игра 'Ловушка слов' начата! Отгадайте слово: %s %s", displayWord, hint))

	select {
	case <-time.After(30 * time.Second):
		s.ChannelMessageSend(m.ChannelID, "Время вышло! Игра 'Ловушка слов' завершена.")
		trapInProgress = false
	case msg := <-waitForUserResponse(s, m.ChannelID):
		checkTrapGuess(s, m, msg.Content)
		trapInProgress = false
	}
}

func waitForUserResponse(s *discordgo.Session, channelID string) <-chan *discordgo.MessageCreate { // waitForUserResponse waits for a response from the user and returns the event channel of the new message.
	messageCreateChan := make(chan *discordgo.MessageCreate, 1)

	// Handler function for the MessageCreate event
	handler := func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.ChannelID == channelID {
			messageCreateChan <- m
		}
	}

	s.AddHandler(handler)

	return messageCreateChan
}

func generateTrapWord() (string, string) { // generateTrapWord generates a random word for the Word Trap game.
	wordHints := map[string]string{
		"apple":  "фрукт",
		"banana": "фрукт",
		"grey":   "цвет",
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var selectedWord string
	for word := range wordHints {
		selectedWord = word
		break
	}

	for word := range wordHints {
		if r.Intn(2) == 0 {
			selectedWord = word
		}
		break
	}

	hint := wordHints[selectedWord]
	return selectedWord, fmt.Sprintf("(Подсказка: %s)", hint)
}

func hideRandomLetters(word string) string { // hides letters
	wordRunes := []rune(word)
	for i := range wordRunes {
		if i != 0 && i != len(wordRunes)-1 {
			wordRunes[i] = '*'
		}
	}
	return string(wordRunes)
}

func checkTrapGuess(s *discordgo.Session, m *discordgo.MessageCreate, guess string) { // check user answer
	if guess == trapWord {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Поздравляю! Вы угадали слово '%s'! Вы выиграли!", trapWord))
		trapInProgress = false
	} else {
		s.ChannelMessageSend(m.ChannelID, "К сожалению, это не правильное слово. Попробуйте еще раз.")
	}
}
