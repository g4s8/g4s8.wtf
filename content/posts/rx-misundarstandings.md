+++ 
date = 2021-02-02T00:00:00Z
title = "Four misunderstandings of reactive streams"
categories = ["Java", "reactive-streams"]
+++

Do you know what [reactive-streams](https://www.reactive-streams.org/)
are used for?

I often hear something like:
  1. It makes code asynchronous
  2. It makes program faster
  3. It's more declarative/readable/maintainable
  4. It's a way for implementing observer pattern

Sometimes I even heard that reactive-streams are used as a
compatibility version of Java 1.8 stream API for
mapping and filtering over collections.
But all of these answers are not really correct.
The main purpose of reactive-streams is utilize more resources
by allowing the data consumer to control the speed of the data-flow
via asynchronous back-pressure. Let me explain what does this mean.


## Roles

Reactive streams has two mandatory roles in a flow, it's:
 - data producer -- someone who generates the data
 - data consumer -- someone who receives the data
 - data processor -- intermediate layer for data processing

They could be any kind of IO resources, such as network socket, file, system APIs;
producer can also generate data on the fly, e.g. RNG or bytes sequence;
consumer can reduce the data: e.g. in case of crypto-consumer it could update
some message digest with received data.
network packets from socket are saved to file; or the producer could be a
file, and a consumer is message-digest: calculating the hash-sum of the file, etc.
Data processor can modify or filter the data. Reactive stream flow requires one consumer
and one producer and optionally any number of processor.


## Back-pressure

One of the core part in the anatomy of reactive streams
[specification](https://github.com/reactive-streams/reactive-streams-jvm/blob/v1.0.3/README.md),
is a non-blocking back-pressure.

The beck-pressure -- is a mechanism for requesting the data **from consumer by producer**.
So the producer actually controls the data flow. E.g. if we're writing from the network socket
to the file using reactive-streams, the file object is requesting next amount of packets from
the socket, and the socket sends it **only** when received the request. Non-blocking back-pressure
means that the producer returns immediately after receiving the request, even if it doesn't have
enough data for sending to the consumer. So the data delivery will be asynchronous, the producer promises to
deliver requested data sometime later if any exist. This non-blocking back-pressure allows the data flow to be
processed at the speed of slowest component in this flow: if the producer is faster than a consumer,
the producer will send the data only after request, so it can't send it faster than data is consumed;
and if the producer is slower than a consumer, then it won't provide some data right after the request,
since, it doesn't have it yet.


## Data oriented

The reactive streams idea is all around the data. It allows to handle data streams more effectively
in a non-blocking way (where possible). Non-blocking should not be confused with asynchronous. It's
two different terms. Non-blocking could not be asynchronous, and asynchronous not always mean non-blocking.
Talking about data processing, the non-blocking terms mean that we perform some IO operation (almost) instantly:
the system returns the control to the user code right after it receives the data for processing.
For instance, we can read a data from socket with blocking and non-blocking ways:
 - the blocking request asks operating system for the next amount of data, and the system waits until it receives
 the data requested to return the control to user code.
 - the nob-blocking requests asks OS to give any data if exists, we don't know how many data OS will return us after
 this call, it could be empty buffer.

When using blocking API, programmers usually creates a new threads for each data processor, because it allows to wait
for multiple blocking IO operation simultaneously. It's quite fine if we are talking about small number of
parallel data streams for processing, but if we're going to handle some big amount of parallel streams, it could become
a problem, since each thread for processing consumes system resources (memory), even if we have unlimited amount of memory,
the thread needs to wait somewhere for blocking operation completion - it consumes CPU for this waiting notifications.

Non-blocking approach allows to process many data streams simultaneously in a one thread:
we just asks OS for the next chunk of data from the hardware, and binding the context to correct data stream.
E.g. in a web server we can load HTTP request data from different connections in a one thread: we bind data processor
to TCP connection, and read the data from socket in a non-blocking way with busy-loop, if we receive next chunk of
data, then we send it to bound processor.

So why do we care about reactive streams with these non-blocking IO API? Because of the slowest consumer.
If implemented correctly, the application may request some data and receive it in a non-blocking way,
let's continue the example with network-server: let's assume we want to write received data into the file,
in this example, the network is very fast but the disk is slow;
the client opens a connection, the server handles new connection by binding
it to the data consumer -- file object, this file opens a descriptor for writing and asks the socket object for next
chunked of data (using async back-pressure), the socket sends `ACK` packets and the clients sends chunk of data,
the socket object periodically checks the buffer for new data in the connection, and when received, it submits it to
the consumer (file), file writes this chunk to the disk and requests next chunk, this could be repeated until the end
of data stream (client closes connection).

As you may notice, it's not really effective, since we have delays between data requesting and writing. It could
be fixed if the consumer will request some data ahead: e.g. a file requests 3 chunks of data instead of one, and
requests next 3 when processing 2-nd of 3. It uses more memory but the consumer will not stay idle in this case.
The strategy for requesting next data could be adaptive and change based on consumer needs.


## Asynchronous?

So is it asynchronous or not? The beck-pressure must be asynchronous.
When we are requesting for next chunk of data, then we don't know exactly when we
receive it. Other communication requirements are not so strict. If the producer
sends some data to the consumer, the consumer may process it asynchronously. Or may not.
It's not specified by reactive streams. Usually data streams are processed sequentially,
but we don't really know that all sequential calls are coming from the same thread.
So the asynchronous data processing is not the mandatory part of the reactive streams.


## Fast?

Does reactive streams makes the program fast? No, it doesn't by default. Only if you compare blocking
and non-blocking algorithms, the non-blocking one could be faster on huge amount of parallel streams,
and it'd be easier to use reactive streams for non-blocking data processing comparing to some custom
implementations. Just using reactive-streams without changing the data flow won't help to improve the
performance.


## Maintainable?

RS is a specification with some popular implementation. These implementations has a quite big
communities, it's easy to find popular issues and questions on StackOverflow or GitHub.
If you compare it with no-name solutions for non-blocking data processing or
with custom algorithms, reactive streams are much maintainable because of that. On the other side,
if you're using it for a wrong reason and not for a data processing, it could make a project maintenance
like a hell.


## Conclusion

Use reactive streams carefully and when applicable. Don't use irrelevantly -- it may make the code harder to
read and slower to work. When using correctly and with proper IO APIs, it increases resource consumption,
and may help to reduce the costs for servers on high loads and implement auto-scaling more efficiently.
It's just a tool for working with a data flow. It's not necessary to spread RS interfaces
across all the application, if this utility is used for data processing only, then the maintenance costs
are not really increased. In next posts I'll show some examples how to use reactive streams and how to implement
it from scratch.

{{< share >}}
