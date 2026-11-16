// web/static/stats.js
// Подгружает данные статистики через fetch и рисует графики Chart.js.

var monthNames = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
var colors = ['#4e79a7', '#f28e2b', '#e15759', '#76b7b2', '#59a14f', '#edc948', '#b07aa1', '#ff9da7', '#9c755f', '#bab0ac'];

var categoryChart = null;
var monthChart = null;

// getJSON запрашивает адрес и возвращает разобранный JSON.
// Если сервер ответил ошибкой (400, 401, 500), бросает исключение с текстом из поля error.
async function getJSON(url) {
    var response = await fetch(url);
    var data = await response.json();
    if (!response.ok) {
        throw new Error(data.error || 'ошибка запроса');
    }
    return data;
}

// Круговая диаграмма: траты по категориям за период
async function loadCategoryChart() {
    var errorBox = document.getElementById('category-error');
    var emptyBox = document.getElementById('category-empty');
    var totalBox = document.getElementById('category-total');
    errorBox.textContent = '';

    var params = new URLSearchParams();
    var from = document.getElementById('from').value;
    var to = document.getElementById('to').value;
    if (from) {
        params.set('from', from);
    }
    if (to) {
        params.set('to', to);
    }

    try {
        var data = await getJSON('/api/stats/by-category?' + params.toString());

        // старый график нужно удалить, иначе Chart.js не даст нарисовать новый на том же canvas
        if (categoryChart) {
            categoryChart.destroy();
            categoryChart = null;
        }

        var hasData = data.items.length > 0;
        emptyBox.hidden = hasData;
        totalBox.textContent = hasData ? 'Всего за период: ' + data.total.toFixed(2) + ' ₽' : '';
        if (!hasData) {
            return;
        }

        categoryChart = new Chart(document.getElementById('category-chart'), {
            type: 'pie',
            data: {
                labels: data.items.map(function (item) { return item.name; }),
                datasets: [{
                    data: data.items.map(function (item) { return item.total; }),
                    backgroundColor: data.items.map(function (item, i) { return colors[i % colors.length]; })
                }]
            }
        });
    } catch (error) {
        errorBox.textContent = 'Не удалось загрузить статистику: ' + error.message;
    }
}

// Столбчатая диаграмма: траты по месяцам выбранного года
async function loadMonthChart() {
    var errorBox = document.getElementById('month-error');
    var emptyBox = document.getElementById('month-empty');
    errorBox.textContent = '';

    var params = new URLSearchParams();
    var year = document.getElementById('year').value;
    if (year) {
        params.set('year', year);
    }

    try {
        var data = await getJSON('/api/stats/by-month?' + params.toString());

        if (monthChart) {
            monthChart.destroy();
            monthChart = null;
        }

        var sum = data.months.reduce(function (acc, m) { return acc + m.total; }, 0);
        emptyBox.hidden = sum > 0;

        monthChart = new Chart(document.getElementById('month-chart'), {
            type: 'bar',
            data: {
                labels: data.months.map(function (m) { return monthNames[m.month - 1]; }),
                datasets: [{
                    label: 'Сумма, ₽',
                    data: data.months.map(function (m) { return m.total; }),
                    backgroundColor: '#4e79a7'
                }]
            },
            options: {
                scales: { y: { beginAtZero: true } },
                plugins: { legend: { display: false } }
            }
        });
    } catch (error) {
        errorBox.textContent = 'Не удалось загрузить статистику: ' + error.message;
    }
}

document.getElementById('category-form').addEventListener('submit', function (event) {
    event.preventDefault(); // не перезагружаем страницу
    loadCategoryChart();
});

document.getElementById('month-form').addEventListener('submit', function (event) {
    event.preventDefault();
    loadMonthChart();
});

// первая загрузка графиков при открытии страницы
loadCategoryChart();
loadMonthChart();
