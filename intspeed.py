#!/usr/bin/env python3

import click
import subprocess
import re
import json
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

VERSION = "1.0.0"


def fetch_city_list():
    output = subprocess.check_output(
        "speedtest-go --city-list", shell=True).decode('utf-8')
    return re.findall(r'\((\w\w)\)\s+([\w\s]+)', output)


def run_speed_test(city_name):
    try:
        print(f"\nStarting speed test for {city_name.strip()}...")
        speedtest_output = subprocess.check_output(
            f"speedtest-go --city='{city_name.strip()}'", shell=True).decode('utf-8')

        download_speed = re.search(
            r'Download: ([\d\.]+)Mbps', speedtest_output)
        upload_speed = re.search(r'Upload: ([\d\.]+)Mbps', speedtest_output)
        latency = re.search(r'Latency: ([\d\.]+)ms', speedtest_output)

        if download_speed and upload_speed and latency:
            print(f"Speed test results for {city_name.strip()}:")
            print(f"  Download Speed: {download_speed.group(1)} Mbps")
            print(f"  Upload Speed: {upload_speed.group(1)} Mbps")
            print(f"  Latency: {latency.group(1)} ms")
            return {
                'city': city_name.strip(),
                'download_speed': float(download_speed.group(1)),
                'upload_speed': float(upload_speed.group(1)),
                'latency': float(latency.group(1))
            }
        else:
            print(f"Speed test failed for {city_name.strip()}")
            return {
                'city': city_name.strip(),
                'error': 'Speed test failed'
            }
    except subprocess.CalledProcessError as e:
        print(
            f"Speed test failed for {city_name.strip()} with error: {str(e)}")
        return {
            'city': city_name.strip(),
            'error': str(e)
        }


def process_data(data):
    df = pd.DataFrame(data)
    numeric_fields = ['download_speed', 'upload_speed', 'latency']
    df[numeric_fields] = df[numeric_fields].apply(
        pd.to_numeric, errors='coerce')
    return df.dropna(subset=numeric_fields)


def draw_plot(df):
    df_melt = df.melt(id_vars='city', value_vars=[
                      'download_speed', 'upload_speed', 'latency'])

    sns.set_theme(style="whitegrid")
    fig, ax = plt.subplots(figsize=(12, 8))

    plt.title('International Speedtest by Rotko Networks',
              fontsize=24, fontweight='bold', y=1.05)

    sns.barplot(x='city', y='value', hue='variable',
                data=df_melt, ax=ax, palette='viridis')

    ax.set_xlabel('City', fontsize=16)
    ax.set_ylabel('Value', fontsize=16)
    ax.legend(title='Metric', title_fontsize='13', fontsize='12')

    plt.xticks(rotation=90)
    plt.tight_layout()
    plt.savefig('speedtest.png', bbox_inches='tight')


@click.group()
def cli():
    pass


@click.command()
def test():
    """Run the speed test."""
    city_list = fetch_city_list()
    results = [run_speed_test(city_name) for city_code, city_name in city_list]

    results = sorted(results, key=lambda x: float(
        x['latency']) if 'latency' in x else float('inf'))

    with open('results.json', 'w') as f:
        json.dump(results, f)


@click.command()
def draw():
    """Draw the results graph from results.json file."""
    with open('results.json', 'r') as f:
        results = json.load(f)
    df = process_data(results)
    draw_plot(df)


@click.command()
def version():
    """Check the version."""
    click.echo(f"International Speedtest CLI, version {VERSION}")
    click.echo(f"from Rotko Networks OU, https://rotko.net")


cli.add_command(test)
cli.add_command(draw)
cli.add_command(version)


if __name__ == "__main__":
    cli()
