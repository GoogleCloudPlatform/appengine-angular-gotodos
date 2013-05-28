//
// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// gotodos is an App Engine JSON backend for managing a todo list.
//
// It supports the following commands:
//
// - Create a new todo
// POST /todos
// > {"text": "do this"}
// < {"id": 1, "text": "do this", "created": 1356724843.0, "done": false}
//
// - Update an existing todo
// POST /todos
// > {"id": 1, "text": "do this", "created": 1356724843.0, "done": true}
// < {"id": 1, "text": "do this", "created": 1356724843.0, "done": true}
//
// - List existing todos:
// GET /todos
// >
// < [{"id": 1, "text": "do this", "created": 1356724843.0, "done": true},
//    {"id": 2, "text": "do that", "created": 1356724849.0, "done": false}]
//
// - Delete 'done' todos:
// DELETE /todos
// >
// <
package gotodos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"appengine"
	"appengine/datastore"
)

func defaultTodoList(c appengine.Context) *datastore.Key {
	return datastore.NewKey(c, "TodoList", "default", 0, nil)
}

type Todo struct {
	Id   int64  `json:"id" datastore:"-"`
	Text string `json:"text" datastore:",noindex"`
	Done bool   `json:"done"`
	Created time.Time `json:"created"`
}

func (t *Todo) key(c appengine.Context) *datastore.Key {
	if t.Id == 0 {
		t.Created = time.Now()
		return datastore.NewIncompleteKey(c, "Todo", defaultTodoList(c))
	}
	return datastore.NewKey(c, "Todo", "", t.Id, defaultTodoList(c))
}

func (t *Todo) save(c appengine.Context) (*Todo, error) {
	k, err := datastore.Put(c, t.key(c), t)
	if err != nil {
		return nil, err
	}
	t.Id = k.IntID()
	return t, nil
}

func decodeTodo(r io.ReadCloser) (*Todo, error) {
	defer r.Close()
	var todo Todo
	err := json.NewDecoder(r).Decode(&todo)
	return &todo, err
}

func getAllTodos(c appengine.Context) ([]Todo, error) {
	todos := []Todo{}
	ks, err := datastore.NewQuery("Todo").Ancestor(defaultTodoList(c)).Order("Created").GetAll(c, &todos)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(todos); i++ {
		todos[i].Id = ks[i].IntID()
	}
	return todos, nil
}

func deleteDoneTodos(c appengine.Context) error {
	return datastore.RunInTransaction(c, func(c appengine.Context) error {
		ks, err := datastore.NewQuery("Todo").KeysOnly().Ancestor(defaultTodoList(c)).Filter("Done=", true).GetAll(c, nil)
		if err != nil {
			return err
		}
		return datastore.DeleteMulti(c, ks)
	}, nil)
}

func init() {
	http.HandleFunc("/todos", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	val, err := handleTodos(c, r)
	if err == nil {
		err = json.NewEncoder(w).Encode(val)
	}
	if err != nil {
		c.Errorf("todo error: %#v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleTodos(c appengine.Context, r *http.Request) (interface{}, error) {
	switch r.Method {
	case "POST":
		todo, err := decodeTodo(r.Body)
		if err != nil {
			return nil, err
		}
		return todo.save(c)
	case "GET":
		return getAllTodos(c)
	case "DELETE":
		return nil, deleteDoneTodos(c)
	}
	return nil, fmt.Errorf("method not implemented")
}
