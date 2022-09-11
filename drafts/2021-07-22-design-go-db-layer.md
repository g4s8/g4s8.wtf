---
title: "Designing database layer in Go"
date: 2021-07-22T10:51:44+03:00
draft: true
slug: ""
tags: ["Go", "Database", "Architecture"]
categories: ["arc"]
externalLink: ""
series: []
---

It's a common problem: how to design a maintainable database storage layer
with imperative languages such as Go? Let's start with a definition of maintainability:
1) the DB package is isolated enough -- modules don't share common data and each module
provides only required data
(aka [low coupling](https://courses.cs.washington.edu/courses/cse403/96sp/coupling-cohesion.html)).
A changes in each module has low or zero impact on another module.
2) DB module hides its internals (strong encapsulation), and prevents another module to depends on
local data which leads to higher coupling between modules.
3) single-purpose units in DB module -- each structure or function has only single purpose and
has only relevant data which is required for some particular operation (high coupling).

Let's define public API first:
```go
package db

// Operation that could be performed in transaction
type Operation interface {
  perform(context.Context, *sql.Tx) error
}

// Perform operations using database
func Perform(db *sql.DB, ops... Operation) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
                return err
	}
	for _, op := range ops {
		if err := op.perform(ctx, tx); err != nil {
			tx.Rollback()
                        return err
		}
	}
	if err := tx.Commit(); err != nil {
                return err
	}
	return nil 
}
```
Here we defined two APIs:
 - An interface to perform operation in database transaction with private `perform` method:
 it allow us to control all operations, since they should be defined in same package, so we
 hide implementation details and reduce potential coupling.
 - Method `Perform` to run operations -- it's the only entry point for database.

Now, we can implement some operations. Let's assume we have a table `users` with fields `id`, `name`,
and email:
```go
type OpGetUser struct {
        id int
        name string
        email string
}

func (op *OpGetUser) Name() string {
        return op.name
}

func (op *OpGetUser) Email() string {
        return op.email
}

func GetUser(id int) *OpGetUser {
        return &opGetUser{id: id}
}

func (op *opGetUser) perform(ctx context.Context, tx *sql.Tx) error {
        rows, err := tx.QueryContext(ctx, `SELECT name, email FROM users WHERE id = $1`, op.id)
        if err != nil {
                return err
        }
        defer rows.Close()
        if rows.Next() {
                if err := rows.Scan(&op.name, &op.email); err != nil {
                        return err
                }
        } else {
                return errors.New("user not found")
        }
        return nil
}
```

So here we hide implementation details behind this `perform` method, and expose
`Name` and `Email` method to get only required data (reducing data coupling here).

The client can use it like:
```go
op := db.GetUser(42)
if err := db.Perform(pool, op); err != nil {
        return err
}
fmt.Printf("user 42 is %d\n", op.Name())
```
