package web

import (
	"encoding/json"
	"fmt"

	"github.com/rotkonetworks/intspeed/pkg/results"
)

func GenerateHTML(testResults *results.TestResults) string {
	stats := testResults.GetStats()
	
	// Convert results to JSON for JavaScript
	jsonData, _ := json.Marshal(testResults.Tests)
	jsonStats, _ := json.Marshal(stats)
	
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>International Speedtest Results</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body class="bg-gray-50 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="bg-white rounded-lg shadow-lg p-6 mb-8">
            <h1 class="text-4xl font-bold text-gray-800 mb-2">🌍 intspeed results</h1>
            <p class="text-gray-600">Global network performance analysis</p>
        </div>

        <!-- Stats Overview -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
            <div class="bg-white rounded-lg p-6 shadow">
                <div class="text-2xl font-bold text-blue-600">%d/%d</div>
                <div class="text-gray-600">Success Rate</div>
                <div class="text-sm text-gray-500">%.1f%%</div>
            </div>
            <div class="bg-white rounded-lg p-6 shadow">
                <div class="text-2xl font-bold text-green-600">%.1f ms</div>
                <div class="text-gray-600">Avg Latency</div>
            </div>
            <div class="bg-white rounded-lg p-6 shadow">
                <div class="text-2xl font-bold text-purple-600">%.1f Mbps</div>
                <div class="text-gray-600">Avg Download</div>
            </div>
            <div class="bg-white rounded-lg p-6 shadow">
                <div class="text-2xl font-bold text-orange-600">%.1f Mbps</div>
                <div class="text-gray-600">Avg Upload</div>
            </div>
        </div>

        <!-- Charts -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
            <div class="bg-white rounded-lg p-6 shadow">
                <h3 class="text-xl font-semibold mb-4">Speed by Location</h3>
                <canvas id="speedChart" width="400" height="200"></canvas>
            </div>
            <div class="bg-white rounded-lg p-6 shadow">
                <h3 class="text-xl font-semibold mb-4">Latency Distribution</h3>
                <canvas id="latencyChart" width="400" height="200"></canvas>
            </div>
        </div>

        <!-- Results Table -->
        <div class="bg-white rounded-lg shadow overflow-hidden">
            <div class="px-6 py-4 border-b">
                <h3 class="text-xl font-semibold">Detailed Results</h3>
            </div>
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead class="bg-gray-50">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Location</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Latency</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Download</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Upload</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200" id="resultsTableBody">
                        <!-- Populated by JavaScript -->
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <script>
        const testData = %s;
        const stats = %s;
        
        // Populate results table
        const tbody = document.getElementById('resultsTableBody');
        testData.forEach(test => {
            const row = tbody.insertRow();
            row.innerHTML = ` + "`" + `
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm font-medium text-gray-900">${test.location.name}</div>
                    <div class="text-sm text-gray-500">${test.location.region}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    ${test.success ? test.latency_ms.toFixed(1) + ' ms' : 'N/A'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    ${test.success ? test.download_mbps.toFixed(1) + ' Mbps' : 'N/A'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                    ${test.success ? test.upload_mbps.toFixed(1) + ' Mbps' : 'N/A'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${test.success ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}">
                        ${test.success ? 'Success' : 'Failed'}
                    </span>
                </td>
            ` + "`" + `;
        });

        // Speed Chart
        const speedCtx = document.getElementById('speedChart').getContext('2d');
        const successfulTests = testData.filter(t => t.success);
        
        new Chart(speedCtx, {
            type: 'bar',
            data: {
                labels: successfulTests.map(t => t.location.name),
                datasets: [{
                    label: 'Download (Mbps)',
                    data: successfulTests.map(t => t.download_mbps),
                    backgroundColor: 'rgba(59, 130, 246, 0.7)',
                }, {
                    label: 'Upload (Mbps)',
                    data: successfulTests.map(t => t.upload_mbps),
                    backgroundColor: 'rgba(245, 158, 11, 0.7)',
                }]
            },
            options: {
                responsive: true,
                scales: {
                    x: { ticks: { maxRotation: 45 } },
                    y: { beginAtZero: true }
                }
            }
        });

        // Latency Chart
        const latencyCtx = document.getElementById('latencyChart').getContext('2d');
        new Chart(latencyCtx, {
            type: 'line',
            data: {
                labels: successfulTests.map(t => t.location.name),
                datasets: [{
                    label: 'Latency (ms)',
                    data: successfulTests.map(t => t.latency_ms),
                    borderColor: 'rgba(16, 185, 129, 1)',
                    backgroundColor: 'rgba(16, 185, 129, 0.1)',
                    tension: 0.4,
                }]
            },
            options: {
                responsive: true,
                scales: {
                    x: { ticks: { maxRotation: 45 } },
                    y: { beginAtZero: true }
                }
            }
        });
    </script>
</body>
</html>`,
		stats.Successful, stats.Total, stats.SuccessRate,
		stats.AvgLatency, stats.AvgDownload, stats.AvgUpload,
		string(jsonData), string(jsonStats))
}
