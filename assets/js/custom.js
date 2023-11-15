(function(location){
  var www = location.hostname.startsWith("www.");
  if (www) {
    location.hostname = location.hostname.substring(4);
    return;
  }

  var debug = location.hostname === "localhost";
  var locationURL = new URL(location);
  var debugParam = locationURL.searchParams.get("debug");
  if (debugParam === "true") {
    debug = true;
  } else if (debugParam === "false") {
    debug = false;
  }

  if (!debug) {
    var dummy = function(){};
    console.log = dummy;
    console.assert = dummy;
    console.debug = dummy;
    console.error = dummy;
    console.info = dummy;
    console.log = dummy;
    console.trace = dummy;
    console.warn = dummy;
  } else {
    console.debug("Debug mode enabled");
  }
})(location || window.location || document.location)
