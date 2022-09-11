---
date: "2019-06-26T00:00:00Z"
draft: true
title: Fear of decomposition
---

I'm observing a global tendency among software people:
most of them prefere to work with big amount of data,
and it doesn't matter who is that people: programmers,
managers, other realted people. Let me explain:
programmer says: don't create new class for new logic, let's
put it all in `Utils` class, manager says: don't create
new tickets, let's use this story-ticket for all related
features, or don't release it every day, we can publish it
once a week, support guys often creates single huge ticket
where describe all different issues received from users, etc.
I don't understand that fear, but I investigated it to find the
seed. In this post I'll show you what I found and will try
to release you from this fear (if you have it).

# Common fears of small classes

There's few reasons why programmers are using big classes
instead many small classes. If you know more, post a comment.


"Too many work just for few lines of code".<br/>
First you need to find a name for class, then create new file,
find a name for method(s), write documentation, encapsulate some state,
create constructor, and put few lines of actual logic into these method(s).
Yes, it takes more time than just write it in utility class as static function,
but I see advantages in all these steps that programmer have to do to
put small piece of logic into new class:
 1. Name - having a good name for the class and methods is a great advantage
 comparing to utility classes or generic huge classes: if you name your class
 correctly, you will be able to understand what that class do from one look, e.g.
 compare that code: `new File("test.txt").write("hello")`
 vs `Utils.writeToFile("test.txt", "hello")`: in first case it's clear that
 you're writting text "hello" to file "test.txt", in second you may need to
 check documentation or source to understand what is first argument and what is
 second.
 2. Documentation: with separate class you have an ability to write the
 documentation for class and for method, but when appending new method to class
 you will extend class documentation what is huge enough already. So you can
 describe **what** did you add (class docs) and what your code **does** (method docs),
 i.e. splitting it into declaration documentation and behavior documentation. Except that
 it helps code reader to better understand the code,
 it requires code writer to think twice before creating new class, what improves code
 quality.
 3. Constructor: it's additional place where documentation can be written, and it's
 easy to add additional constructor for unit testing with stubs, that would look ugly
 with utility classes, but looks good with constructors.
 4. Encapsulation: it's also very improtant point - programmer have to think what is
 the state and what should be method parameters, and state can be mocked for testing.

I think it was fixed
"Don't know how to name" / "Hard to find class later".<br/>
The most popular issues with small classes all are about
naming. It's easier to append some logic to existing class,
and design all classes to be huge - make its scope as big as
possible, e.g. if you have `MainController` in a web app, it'll
be easy to add almost any method at least somehow related to web
layer. So many people prefer to give generic names to classes
to append more methods later. Another issue with many small classes
is about inability to find it later - if you create many classes with
poor names, it'll be really hard to find it, the only solution is to
give distinct name to new class, but who whant to spend working hours
on class naming thoughts if it's possible to write all new logic in
existing class, right?


"Class instantiation is slow".<br/>
Many nowaday progammers were learning programming languages
some times ago, and they bring all best practices from 200X-201X
to present days. E.g. Google suggested to avoid using instance methods,
because they are slow, and use static functions for Android 2.X in 2010 y.
And many other similar examples, e.g. old JVMs performed faster with static
methods, etc. There're a lot of blog posts and SO answers from that time,
which suggested to avoid instantiating new class if it's not necessary.
newcomers are reading these posts and thinking about minor performance
improvements instead of maintainability. There's one good quote from D. Knuth:
> Programmers waste enormous amounts of time thinking about, or worrying about,
> the speed of noncritical parts of their programs,
> and these attempts at efficiency actually have a strong negative impact
> when debugging and maintenance are considered.
> We should forget about small efficiencies, say about 97% of the time:
> premature optimization is the root of all evil.
> Yet we should not pass up our opportunities in that critical 3%.

And that's true, if your app have some performance issues, most probably (99%)
it was caused by your code, not Java constructors or non-static methods.
Maintainability first.

# Arguments against micro tasks

