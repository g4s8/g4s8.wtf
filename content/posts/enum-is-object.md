+++ 
draft = false
date = 2020-12-26T00:00:00Z
title = "Enum: Object or global variable?"
description = "My thoughts on enum role in Java programming"
slug = "enum-is-object" 
categories = ["Java", "OOP"]
aliases = ["/posts/2020-12-26-enum-objects/"]
+++

Everybody knows that global variables are evil. In case of global constants it's bad
because the constant with the global scope creates strong dependency for a component
on the **data** stored in this variable. If a global variable is an object, then it leads
to tight coupling between client's code and this object (strong dependency again).
Java developers uses `enum`s frequently as a container for global constants.
But what if we think about it from the different point of view? Maybe it's possible to
use `enum`s as real objects but not as a dumb constants container?

## What's wrong?

This is an example of common Java `enum` which coopts all bad
practives of global constants.

Assume we have CLI app where user can specify different naming policies
to store some files in local file system:
 - plain names = same as default item names
 - append SHA256 checksum to file name
 - append SHA1 checksum to file name
 - other policies could be added later

This requirements I took from real practice.

```java
// All supported naming policies
public enum NamingPolicy {
  PLAIN,
  SHA256,
  SHA1;
}

// some data item which can be saved to local file
class FileItem {

  // some data that should be saved locally
  private final Content data;

  // constructor FileItem(Content) ommited

  // save content to `dir` using `policy` for file names
  void save(Path dir, NamingPolicy policy) {
    switch (policy) {
      case PLAIN:
        Files.write(
          dir.resolve(data.name()), data.bytes()
        );
        break;
      case SHA256:
      case SHA1:
        final byte[] bytes = data.bytes();
        String hash = hex(checksum(policy.name(), bytes));
        Files.write(
          dir.resolve(hash + data.name()),
          data.bytes()
        );
        break;
      default:
        throw new UnsupportedException(
          "Unknown naming policy: " + policy
        );
    }
  }
}
```

The main problems with this code:
 - strong dependency on `NamingPolicy` constants.
 - complexity of `save()` method depends on amount of
 policies, each new policy implementation require changes
 in `save()`, this method is like a main dispatcher of
 all scenarios, it's responsible for all known policy processing.
 - it's hard to test the behavior of `save()`;
 it has many behaviors, test method should cover all possible
 branches to verify it. If new policy will be added but not tested,
 then it will be easy to see passed tests for broken code.

## How to change it?

All of the issue can be easilly fixed by replacing `enum` with
interface with respective implementations:
```java
interface NamingPolicy {

  // File name for given source of file and data.
  String name(String src, byte[] data);
}

// name is a given source name
class Plain implements NamingPolicy {
  @Override
  public String name(String src, byte[] data) {
    return src;
  }
}

// name is a SHA256 of content data
class Sha256 implements NamingPolicy {
  @Override
  public String name(String src, byte[] data) {
    return src + hex(checksum("SHA-256", data));
  }
}

// name is a SHA1 of content data
class Sha1 implements NamingPolicy {
  @Override
  public String name(String src, byte[] data) {
    return src + hex(checksum("SHA-1", data));
  }
}

```
Now `FileItem` class looks like this:
```java
// some data item which can be saved to local file
class FileItem {

  // some data that should be saved locally
  private final Content data;

  // constructor FileItem(Content) ommited

  // save content to `dir` using `policy` for file names
  void save(Path dir, NamingPolicy policy) {
    Files.write(
      dir.resolve(policy.name(data.name(), data.bytes())),
      data.bytes()
    );
  }
}
```

All problems of global constants are solved:
 - `save()` methods depends on abstraction,
 the coupling is low
 - It's easy to introduce new naming policy
 by creating an implementation of the interface.
 The responsibility of `save()` method was narrowed down
 to saving logic only
 - The test for `save()` method covers all possible
 scenarios, since it doesn't depend on policies implementations

## Standard enums

But what if we move all these interface implementations to enum values?
They don't really have any state, just a naked behavior and could be
organized as `enum` decorators:

```java
interface NamingPolicy {

  // File name for given source of file and data.
  String name(String src, byte[] data);
}

// Appends specified digest
// of content to source name
class HashNames implements NamingPolicy {
  // digest API
  private final MessageDigest digest;

  // constructor ommited

  @Override
  public String name(String src, byte[] data) {
    final MessgeDigest copy =
      (MessageDigest) this.digest.clone();
    copy.update(data);
    return src + hex(copy.digest());
  }
}

// predefined standard policies in app domain
enum StandardPolicies implements NamingPolicy {
  // File name is a source name
  PLAIN((src, data) -> src),
  // Appends SHA1 hash of data to source name
  SHA1(new HashNames(MessageDigest.getInstance("SHA-1"))),
  // Appends SHA256 hash of data to source name
  SHA256(new HashNames(MessageDigest.getInstance("SHA-256"));

  private final NamingPolicy policy;

  StandardPolicies(final NamingPolicy policy) {
    this.policy = policy;
  }

  @Override
  public String name(final String src, final byte[] data) {
    return this.policy.name(src, data);
  }
}
```

So we've defined all standard naming policies for application,
the developer can easy understand now that there are 3 standard
policies in domain. Also, it's easy to parse enum values
from string, e.g. if want to pass naming policy as CLI argument,
we can transform it to one of the standard policies from enum values
by using `StandardPolicies.valueOf(param)`.

## Conclusion

Java enums are not just a bag for global constants, enums can implement interfaces
and behave like a real objects. It's quite friendly to group standard objects in
application domain to single enum instance. It's easy to use but we still have
enough flexibility to implement interface by other classes. So don't afraid enums
just because it's often used wrong, use it carefully and get all benefits in your code.
