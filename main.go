package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Secret Структура для хранения секретов
type Secret struct {
	SecretText string
	CreatedAt  time.Time
	Used       bool
}

// Глобальная мапа для хранения секретов по токену
var secrets = make(map[string]Secret)

// Мьютекс для безопасного доступа к мапе секретов
var mu sync.Mutex

// Глобальные переменные для хранения протокола, хоста и пути
var generatedPath string
var generatedFullURL string

// Генерация уникального токена
func generateToken() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Обработчик POST запроса для создания одноразовой ссылки на секрет.
func generateHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		return
	}
	secretText := r.Form.Get("secret")

	// Генерация уникального токена для секрета
	token := generateToken()

	// Получение протокола из заголовка X-Forwarded-Proto
	protocol := r.Header.Get("X-Forwarded-Proto")
	if protocol == "" {
		// Если заголовок отсутствует, используем протокол из r.URL.Scheme
		protocol = r.URL.Scheme
	}

	// Формирование полной ссылки
	generatedPath = fmt.Sprintf("/retrieve/%s", token)
	generatedFullURL = fmt.Sprintf("http://%s%s", r.Host, generatedPath)

	// Сохранение секрета и его токена
	mu.Lock()
	secrets[token] = Secret{SecretText: secretText, CreatedAt: time.Now(), Used: false}
	mu.Unlock()

	// Формирование и отправка JSON-ответа с одноразовой ссылкой
	response := struct {
		Link string `json:"link"`
	}{
		Link: generatedFullURL,
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

// Обработчик GET запроса для отображения формы ввода секрета.
func indexHandler(w http.ResponseWriter, _ *http.Request) {

	// Отправка HTML-страницы с формой ввода
	tmpl, err := template.ParseFiles("/app/index.html")
	if err != nil {
		fmt.Println("Error parsing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Использование шаблона index.html
	err = tmpl.Execute(w, nil)
	if err != nil {
		fmt.Println("Error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// Обработчик GET запроса для отображения секрета по одноразовой ссылке.
func retrieveHandler(w http.ResponseWriter, r *http.Request) {
	// Получение токена из URL
	token := strings.TrimPrefix(r.URL.Path, "/retrieve/")

	// Поиск записи в мапе по токену
	mu.Lock()
	secret, ok := secrets[token]
	if ok && !secret.Used {
		// Отметка секрета как использованного
		secrets[token] = Secret{SecretText: secret.SecretText, CreatedAt: secret.CreatedAt, Used: true}
		// Удаление секрета из мапы
		delete(secrets, token)
	}
	mu.Unlock()

	if !ok || secret.Used {
		// Передача данных для вывода сообщения об ошибке в retrieve.html
		data := struct {
			Secret string
			Used   bool
		}{
			Secret: "Secret not found or already used",
			Used:   true,
		}

		// Использование шаблона retrieve.html
		tmpl, err := template.ParseFiles("/app/retrieve.html")
		if err != nil {
			fmt.Println("Error parsing template:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Отправка HTML-страницы с сообщением об ошибке
		err = tmpl.Execute(w, data)
		if err != nil {
			fmt.Println("Error executing template:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	// Передача данных для вывода секрета в retrieve.html
	data := struct {
		Secret string
		Used   bool
	}{
		Secret: secret.SecretText,
		Used:   false,
	}

	// Использование шаблона retrieve.html
	tmpl, err := template.ParseFiles("/app/retrieve.html")
	if err != nil {
		fmt.Println("Error parsing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Отправка HTML-страницы с секретом
	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println("Error executing template:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func main() {
	// Флаги для установки порта и хостнейма
	port := flag.String("port", "8080", "HTTP server port")
	host := flag.String("host", "", "HTTP server hostname")
	flag.Parse()

	// Формирование адреса для прослушивания
	address := *host + ":" + *port

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/generate", generateHandler)
	http.HandleFunc("/retrieve/", retrieveHandler)

	fmt.Printf("Server is running on %s...\n", address)
	err := http.ListenAndServe(address, nil)
	if err != nil {
		return
	}
}
