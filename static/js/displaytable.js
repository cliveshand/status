// deals with hiding and revealing a table

// takes an id (CSS selector) from the table selected of type String
function displayTable(tableId) {
    var tableSelected = document.getElementById(tableId);
    if (tableSelected.style.display === "none") {
        tableSelected.style.display = "block";
    } else {
        tableSelected.style.display = "none";
    }
}

// expects to receive an HTML Node for a button and changes the
// display text
function swapButtonHTMLText(buttonHTMLElement, communityPortfolioUseCaseName) {
    if (buttonHTMLElement.innerHTML.includes("Collapse")) {
        buttonHTMLElement.innerHTML = "Expand - " + communityPortfolioUseCaseName;
    } else {
        buttonHTMLElement.innerHTML = "Collapse - " + communityPortfolioUseCaseName;
    }
}

// drives the state changes for updating the nodes in the DOM accordingly
// for hiding and revealing HTML tables
function alterTableDisplay(tableId, buttonHTMLElement, communityPortfolioUseCaseName) {
    displayTable(tableId);
    swapButtonHTMLText(buttonHTMLElement, communityPortfolioUseCaseName);
}
