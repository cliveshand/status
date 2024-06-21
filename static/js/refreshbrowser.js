// reload the browser directly
function refreshBrowser() {
    location.reload();
}

// timer to automatically refresh the browser for a user
document.addEventListener("DOMContentLoaded", function() {
    // 5 min in milliseconds
    var refreshTime = 300000;

    setTimeout(refreshBrowser, refreshTime);

});
