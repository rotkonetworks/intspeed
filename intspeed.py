#!/usr/bin/env ./venv/bin/python3

import click
import subprocess
import json
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
from pathlib import Path

# Globals
VERSION = "1.2.0"
DEFAULT_THREADS = 1
RAW_DATA_DIR = Path('raw_data')
RAW_DATA_DIR.mkdir(exist_ok=True)

# Define the CLI group here
@click.group()
def cli():
    """A CLI tool for running and visualizing speed tests."""
    pass

def fetch_city_list():
    """Fetches the list of cities supported by speedtest-go."""
    try:
        output = subprocess.check_output(["./speedtest-go", "--city-list"], text=True)
        cities = {}
        for line in output.splitlines():
            if line.startswith('('):
                parts = line.split('\t')
                if len(parts) >= 2:
                    code, name = parts[0].strip(), parts[1].strip()
                    cities[name.lower()] = code[1:-1]
        return cities
    except subprocess.CalledProcessError as e:
        click.echo(f"Error fetching city list: {e}")
        return {}

def run_speed_test(city_name):
    """Runs a speed test for the given city with 1 thread and saves the raw JSON data."""
    try:
        output = subprocess.check_output(["./speedtest-go", "--city", city_name, "--thread", "1", "--json"], text=True)
        data = json.loads(output)
        file_path = RAW_DATA_DIR / f"{city_name.replace(' ', '_')}.json"
        with open(file_path, 'w') as f:
            json.dump(data, f, indent=2)
        click.echo(f"Test for {city_name} completed and saved.")
    except subprocess.CalledProcessError as e:
        click.echo(f"Speed test failed for {city_name} with error: {e}")

def process_raw_data():
    data = []
    for file in RAW_DATA_DIR.glob('*.json'):
        with open(file, 'r') as f:
            result = json.load(f)
            city = result.get('servers', [{}])[0].get('name', 'Unknown')
            dl_speed_mbps = result.get('servers', [{}])[0].get('dl_speed', 0)
            ul_speed_mbps = result.get('servers', [{}])[0].get('ul_speed', 0)
            latency_ms = result.get('servers', [{}])[0].get('latency', 0) / 1e6
            data.append({'City': city, 'Download Speed (Mbps)': dl_speed_mbps, 'Upload Speed (Mbps)': ul_speed_mbps, 'Latency (ms)': latency_ms})
    return pd.DataFrame(data)


def draw_grouped_bars_with_line(df):
    # Sort DataFrame by Latency in ascending order for a logical plot order
    df_sorted = df.sort_values(by='Latency (ms)', ascending=True).reset_index()

    # Set up the matplotlib figure and axes
    fig, ax1 = plt.subplots(figsize=(12, 8))

    # Create an array with the position of each bar along the x-axis
    bar_width = 0.35  # Width of the bars
    r1 = range(len(df_sorted))
    r2 = [x + bar_width for x in r1]

    # Plot the bars for Download and Upload speeds
    ax1.bar(r1, df_sorted['Download Speed (Mbps)'], color='skyblue', width=bar_width, edgecolor='white', label='Download Speed')
    ax1.bar(r2, df_sorted['Upload Speed (Mbps)'], color='orange', width=bar_width, edgecolor='white', label='Upload Speed')

    # Set the labels, title, and custom x-axis tick labels
    ax1.set_xlabel('City', fontweight='bold')
    ax1.set_ylabel('Speed (Mbps)', fontweight='bold')
    ax1.set_title('Internet Speed Test Results by City')
    ax1.set_xticks([r + bar_width/2 for r in range(len(df_sorted))])
    ax1.set_xticklabels(df_sorted['City'], rotation=45, ha='right')

    # Create secondary Y-axis for latency
    ax2 = ax1.twinx()
    ax2.plot(df_sorted['City'], df_sorted['Latency (ms)'], color='green', marker='o', label='Latency (ms)')
    ax2.set_ylabel('Latency (ms)', fontweight='bold')

    # Adding legend for clarity
    ax1.legend(loc='upper left')
    ax2.legend(loc='upper right')

    # Annotate bars with the numerical data
    for idx, val in enumerate(df_sorted['Download Speed (Mbps)']):
        ax1.text(idx - bar_width/2, val, f'{val:.2f}', ha='center', va='bottom')
    for idx, val in enumerate(df_sorted['Upload Speed (Mbps)']):
        ax1.text(idx + bar_width/2, val, f'{val:.2f}', ha='center', va='bottom')

    # Annotate line plot (latency) with numerical data
    for i, point in df_sorted.iterrows():
        ax2.text(i, point['Latency (ms)'], f"{point['Latency (ms)']:.2f}", ha='center', va='bottom')

    plt.tight_layout()
    plt.savefig(RAW_DATA_DIR / 'speedtest_results_grouped_bar_with_latency.png')
    plt.show()

# Replace the existing plot() function in the CLI with this one
@cli.command()
def plot():
    """Generates a plot from raw data."""
    df = process_raw_data()
    if df.empty:
        click.echo("No data available to plot. Please run the tests first.")
    else:
        draw_grouped_bars_with_line(df)


@cli.command()
def plot():
    """Generates a plot from raw data."""
    df = process_raw_data()
    if df.empty:
        click.echo("No data available to plot. Please run the tests first.")
    else:
        draw_grouped_bars_with_line(df)

@cli.command()
def test():
    """Runs speed tests for all available cities."""
    cities = fetch_city_list()
    for city in cities.keys():
        run_speed_test(city)

if __name__ == "__main__":
    cli()
