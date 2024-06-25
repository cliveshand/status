// status weight priorities as defined in our Slack
const statusPriority = {
    "down": 0,
    "degraded": 1,
    "unknown": 2,
    "operational": 3
};

// how we go through to sort the results from Prometheus/Mimir
// this is only on the side of the client; the data itself is NOT sorted otherwise
function sortTable(tableId, columnIndex) {
    // console.log(tableId);
    var table, rows, switching, i, x, y, shouldSwitch;
    table = document.getElementById(tableId);
    // console.log("table:", table)
    switching = true;
    while (switching) {
        switching = false;
        rows = table.rows;
        // console.log("rows:", rows)
        for (i = 0; i < rows.length - 1; i++) {
            shouldSwitch = false;
            x = rows[i].getElementsByTagName("td")[columnIndex];
            // console.log("x:", x)
            y = rows[i + 1].getElementsByTagName("td")[columnIndex];
            // console.log("y:", y)
            if (statusPriority[x.innerHTML.toLowerCase()] > statusPriority[y.innerHTML.toLowerCase()]) {
                shouldSwitch = true;
                break;
            }
        }
        if (shouldSwitch) {
            // console.log("swapping rows...")
            rows[i].parentNode.insertBefore(rows[i + 1], rows[i]);
            switching = true;
        }
    }
}

// when the window loads, simulate the user clicking on the Status column to sort
window.onload = function() {
    var tables = document.getElementsByTagName("table");
    // we want to skip the first table since it is just an explanation
    for (var i = 1; i < tables.length; i++) {
        // console.log("got tables!!!");
        var headers = tables[i].getElementsByTagName("th")[1];
        // console.log("headers:", headers)
        headers.click();
    }
}