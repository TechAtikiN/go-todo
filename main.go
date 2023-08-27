package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var rnd *renderer.Render
var db*mgo.Database

const (
	hostName  	string = "localhost:27017"
	dbName			string = "todo-app"
	collectionName  string = "todos"
	port 				string = ":9000"
)

type (
	todoModel struct {
		ID 			bson.ObjectId `bson:"_id,omitempty"`
		Title 	string				`bson:"title"`
		Completed bool				`bson:"completed"`
		CreatedAt time.Time		`bson:"createdAt"`
	}

	todo struct {
		ID 			string `json:"id"`
		Title 	string `json:"title"`
		Completed bool `json:"completed"`
		CreatedAt string `json:"createdAt"`
	}
)

func init () {
	rnd = renderer.New()
	session, err := mgo.Dial(hostName)
	checkErr(err)
	session.SetMode(mgo.Monotonic, true)
	db = session.DB(dbName)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkErr(err)
}

func getAllTodo (w http.ResponseWriter, r *http.Request) {
	todos := []todoModel{}

	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Failed to get todos",
			"error": err,
		})
		return
	}
	todoList := []todo{}

		for _, t := range todos {
			todoList = append(todoList, todo{
				ID: t.ID.Hex(),
				Title: t.Title,
				Completed: t.Completed,
				CreatedAt: t.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		rnd.JSON(w, http.StatusOK, renderer.M{
			"data": todoList,
		})
}

func createTodo (w http.ResponseWriter, r *http.Request) {
	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	if t .Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Title is required",
		})
		return
	}

	todo := todoModel{
		ID: bson.NewObjectId(),
		Title: t.Title,
		Completed: false,
		CreatedAt: time.Now(),
	}

	if err := db.C(collectionName).Insert(todo); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Failed to create todo",
			"error": err,
		})
		return
	}

	rnd.JSON(w, http.StatusCreated, renderer.M{
		"message": "Todo created successfully",
		"data": todo,
	})
}

func deleteTodo (w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid id",
		})
		return
	}

	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(id)); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Failed to delete todo",
			"error": err,
		})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo deleted successfully",
	})
}

func updateTodo (w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid id",
		})
		return
	}

	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	if t .Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Title is required",
		})
		return
	}

	if err := db.C(collectionName).Update(
		bson.M{"_id": bson.ObjectIdHex(id)},
		bson.M{"$set": bson.M{
			"title": t.Title,
			"completed": t.Completed,
		}},
	); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Failed to update todo",
			"error": err,
		})
		return
	}
}

func main() {
	stopChannel := make(chan os.Signal)
	signal.Notify(stopChannel, os.Interrupt)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandlers())

	server := &http.Server{
		Addr: port,
		Handler: r,
		ReadTimeout: 60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Println("Listening on port", port)
		if err := server.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	<-stopChannel
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	server.Shutdown(ctx)
	defer cancel()
	log.Println("Server gracefully stopped")
}

func todoHandlers() http.Handler {
	rg :=chi.NewRouter()
	rg.Group(func(r chi.Router) {
		r.Get("/", getAllTodo)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})

	return rg
}