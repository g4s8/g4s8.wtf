---
title: "Implementing reactive producer from scratch"
date: 2021-12-14T16:00:46+03:00
draft: true
---
In this post I explain how to create reactive data producer from scratch using Java.
In many cases you don't need to implement reactive primitives by self because
there are a lot of different libraries and toolkits for that, but this knowledge
will help you understand how reactive streams works under the hood. Also,
there are a lot of use-cases not covered by libraries, so you may want to implement
new reactive library to covert that.

Some background: [reactive-streams](https://www.reactive-streams.org/) specification
was included into JDK9 release as
[java.util.concurrent.Flow](https://docs.oracle.com/javase/9/docs/api/java/util/concurrent/Flow.html),
for backward compatibility we'll use reactive-streams library dependency `org.reactivestreams:reactive-streams:1.0.3`
and `org.reactivestreams:reactive-streams-tck:1.0.3` test compatibility kit (TCK) for unit-testing. To add it with Maven use:
```
<dependencies>
  <dependency>
    <groupId>org.reactivestreams</groupId>
    <artifactId>reactive-streams</artifactId>
    <version>1.0.3</version>
  </dependency>
  <dependency>
    <groupId>org.reactivestreams</groupId>
    <artifactId>reactive-streams-tck</artifactId>
    <version>1.0.3</version>
    <scope>test</scope>
  </dependency>
</dependencies>
```
This `reactive-streams` dependency includes only interfaces, same as in `java.util.concurrent.Flow` package.
Three primary interfaces here are:
 * `Publisher` - reactive data producer.
 * `Subscriber` - reactive data consumer.
 * `Subscription` - connection between producer and consumer, used to request next data chunks or cancel the stream.

Also, it includes `Processor`, which acts as both a consumer and producer, but we'll discuss it in next posts.
For better understanding Java specification of reactive-streams check this
[spec documentation](https://github.com/reactive-streams/reactive-streams-jvm/blob/v1.0.3/README.md#specification)

### The task

For this example I'll show you how to implement reactive data reader from file. For reference, you can open
[github.com/cqfn/rio](https://github.com/cqfn/rio/) repository with this implementation, it may have some additional
modification, we'll discuss only primary details here. The target entry point class here will be `ReadableChannelPublisher` -
it's a data producer which implements `Publisher<ByteBuffer>` interface and can create `ReadableByteChannel` from `java.nio`.


### When a publisher subscribes

The only purpose of any reactive publisher is correctly subscribe, since it's the only method of `Publisher` interface:
```java
public interface Publisher<T> {
    public void subscribe(Subscriber<? super T> s);
}
```

In this method publisher must perform some validation, then run initialization logic and notify subscriber that it's ready
for producing the data. According to [publisher specification](https://github.com/reactive-streams/reactive-streams-jvm/blob/v1.0.3/README.md#1-publisher-code)
1.9 it must call `onSubscribe` method of `Subscriber` after initialization and return normally, except when `Subscriber` is null.
In our case we need to get NIO channel from supplier, notify failure if we failed to get it, on success we call `onSubscribe`:
```java
@Override
public void subscribe(final Subscriber<? super ByteBuffer> subscriber) {
    // acording to spec 1.9 we must throw NPE if subscriber is null
    Objects.requireNonNull(subscriber, "Subscriber can't be null");
    final ReadableByteChannel chan;
    try {
        chan = this.src.channel();
    } catch (final IOException err) {
        // according to 1.2, 1.4 and 1.9 we must first subscribe (with dummy subscription),
        // then signal error and return normally.
        subscriber.onSubscribe(ReadableChannelPublisher.DUMMY);
        subscriber.onError(err);
        return;
    }
    // ReadSubscriberState is a helper decorator for Subscriber, it helps
    // implementing spec 1.6 and 1.8 by remembering cancellation state
    final ReadSubscriberState<? super ByteBuffer> wrap = new ReadSubscriberState<>(subscriber);
    // calling onSubscribe
    wrap.onSubscribe(
        // ReadSubscription is responsible for handling subscriber requests, we'll discuss it later
        new ReadSubscription(
            wrap, this.buffers,
            // Queue implementation for channel read tasks
            new ReadTaskQueue(wrap, chan, this.exec)
        )
    );
}
```

### My subscriptions

When the publisher called `onSubscribe`, it provided `Subscription` implementation as method argument.
`Subscription` defines two methods:
```java
public interface Subscription {
    public void request(long n);
    public void cancel();
}
```
It's controlled by consumer (`Subscriber`), the consumer calls `request(n)` when it want to receive `n` elements
(it may use `Long.MAX_VALUE` to request infinity elements); and it calls `cancel()` when it wants to stop
the stream.

In our implementation we delegate this requests to task queue: we ask producer to read `n` chunks from channel
and send it to consumer.

*It's important to remember, that `request` call must be non-blocking: the consumer requests
`n` items and this method returns immediatelly, and after some time, the producer may perform operations and provide
'less than or equal to' amount of requested elements.*

So we implement `request` method like this:
```java
@Override
public void request(final long count) {
    // check if subscription is done or cancelled, see spec 2.6
    if (this.sub.done()) {
        return;
    }
    // according to 2.9 spec we must signal with exception in case of <= 0 amount
    if (count <= 0) {
        this.queue.clear();
        this.sub.onError(
            new IllegalArgumentException(String.format("Requested %d items", count))
        );
    } else {
        // add reading task to queue and return immediatelly
        this.queue.accept(new ReadRequest.Next(this.sub, this.buffers, count));
    }
}
```

The `ReadRequest` ([src](https://github.com/cqfn/rio/blob/master/src/main/java/org/cqfn/rio/channel/ReadRequest.java))
is our workhorse: it performs reading logic from channel to buffer and sends this buffer to `Subscriber` than.
The `ReadTaskQueue` ([src](https://github.com/cqfn/rio/blob/master/src/main/java/org/cqfn/rio/channel/ReadTaskQueue.java))
accepts such requests from `Subscription` and perform it asynchronously in FIFO order.

You may remember that `request` method must be non-blocking. Actually it can perform some operation (almost)
immediately, e.g. incrementing a counter; or start it in background and return after that.
In our case we add read request to queue structure and then check if the worker thread is running already; if not,
we start background task to process the queue in loop:
```java
public void accept(final ReadRequest request) {
    // check if subscription was cancelled
    if (this.sub.done()) {
        return;
    }
    // add request item to queue
    this.queue.add(request);
    // check if 
    if (this.running.compareAndSet(false, true)) {
        this.exec.execute(
            new ErrorOnException(
                new CloseChanOnError(this, this.channel),
                this.sub
            )
        );
    }
}
```
