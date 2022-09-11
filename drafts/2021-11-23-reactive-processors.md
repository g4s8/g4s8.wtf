---
title: "2021 11 23 Reactive Processors"
date: 2021-11-23T10:45:16+03:00
draft: true
---

How to create reactive processor (publisher + subscriber)

```
<versions.reactive-streams>1.0.3</versions.reactive-streams>
<dependencies>
  <dependency>
    <groupId>org.reactivestreams</groupId>
    <artifactId>reactive-streams</artifactId>
    <version>${versions.reactive-streams}</version>
  </dependency>
</dependencies>
```

Example: reactive compression library, use standard Java classes to compress
and reactive streams of `Publisher<ByteBuffer>`.

Processor has upstream and downstream: downstream is a raw data producer,
upstream is a compressed data consumer.

# Impl.
org.reactivestreams.Processor

`implements Processor<ByteBuffer, ByteBuffer>`

Implement publisher subscription:
```
// inside Processor<> impl

final AtomicLong demand;

static final class Sub implements Subscription {
  private final AtomicLong demand;

  @Override
  public void request(final long req) {
    if (req < 0) {
      throw new IllegalArgumentException("invalid subscription request");
    }
    if (req == 0) {
      return;
    }
    this.demand.updateAndGet(x -> {
      if (x == Long.MAX_VALUE || req == Long.MAX_VALUE || x + req < 0) {
        return Long.MAX_VALUE;
      }
      return x + req;
    });
  }
}

@Override
public void subscribe(Subscriber<? super ByteBuffer> subscriber) {
  subscriber.onSubscribe(new Sub(this.demand));
}
```

demand - is a memory for requested chunkds by downstream
