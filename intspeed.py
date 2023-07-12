#!/usr/bin/env python3

import click
import subprocess
import re
import json
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
from operator import itemgetter

VERSION = "1.1.1"
RETRY_COUNT = 2
DEFAULT_THREADS = 8


def fetch_city_list():
    output = subprocess.check_output(
        "speedtest-go --city-list", shell=True).decode('utf-8')
    return re.findall(r'\((\w\w)\)\s+([\w\s]+)', output)


def run_speed_test(city_name, threads, multiserver, retries=3):
    for _ in range(retries):
        try:
            print(f"\nStarting speed test for {city_name.strip()}...")
            multiserver_flag = "-m" if multiserver else ""
            speedtest_output = subprocess.check_output(
                f"speedtest-go {multiserver_flag} -t {threads} --city='{city_name.strip()}'", shell=True).decode('utf-8')

            download_speed = re.search(
                r'Download: ([\d\.]+)Mbps', speedtest_output)
            upload_speed = re.search(
                r'Upload: ([\d\.]+)Mbps', speedtest_output)
            latency = re.search(r'Latency: ([\d\.]+)ms', speedtest_output)

            if download_speed and upload_speed and latency:
                print(f"Speed test results for {city_name.strip()}:")
                print(f"  Download Speed: {download_speed.group(1)} Mbps")
                print(f"  Upload Speed: {upload_speed.group(1)} Mbps")
                print(f"  Latency: {latency.group(1)} ms")
                return {
                    'city': city_name.strip(),
                    'download_speed': round(float(download_speed.group(1))),
                    'upload_speed': round(float(upload_speed.group(1))),
                    'latency': round(float(latency.group(1))),
                }
            else:
                print(f"Attempt failed for {city_name.strip()}, retrying...")
                continue
        except subprocess.CalledProcessError as e:
            print(
                f"Attempt failed for {city_name.strip()} with error: {str(e)}, retrying...")

    print(
        f"Speed test failed for {city_name.strip()} after {retries} attempts.")
    return {
        'city': city_name.strip(),
        'error': 'Speed test failed after multiple attempts'
    }


def process_data(data):
    df = pd.DataFrame(data)
    numeric_fields = ['download_speed', 'upload_speed', 'latency']
    df[numeric_fields] = df[numeric_fields].apply(
        pd.to_numeric, errors='coerce')
    return df.dropna(subset=numeric_fields).sort_values('latency')


def draw_plot(df, subtitle):
    image_name = re.sub(r'\W+', '_', subtitle.lower()) + \
        ".png" if subtitle else 'speedtest.png'
    df_melt = df.melt(id_vars='city', value_vars=[
        'download_speed', 'upload_speed'])

    sns.set_theme(style="whitegrid")
    fig, ax = plt.subplots(figsize=(12, 8))

    plt.title('International Speedtest by Rotko Networks',
              fontsize=24, fontweight='bold', y=1.05)

    if subtitle:
        plt.figtext(0.5, 0.01, subtitle, wrap=True,
                    horizontalalignment='center', fontsize=12)

    barplot = sns.barplot(x='city', y='value', hue='variable',
                          data=df_melt, ax=ax, palette='viridis')

    ax.set_xlabel('City', fontsize=16)
    ax.set_ylabel('Speed (Mbps)', fontsize=16)
    ax.legend(title='Metric', title_fontsize='13', fontsize='12')

    for p in barplot.patches:
        barplot.annotate(format(p.get_height(), '.0f'),
                         (p.get_x() + p.get_width() / 2., p.get_height()),
                         ha='center', va='center',
                         size=10,
                         xytext=(0, -12),
                         textcoords='offset points')

    plt.xticks(rotation=90)
    plt.tight_layout()
    plt.savefig(image_name, bbox_inches='tight')


@click.group()
def cli():
    pass


@click.command()
@click.option('--threads', default=DEFAULT_THREADS, help='Number of threads to use for speedtest.')
@click.option('--multiserver', default=False, is_flag=True, help='Use multi-server mode for speedtest.')
def test(threads, multiserver):
    """Run the speed test."""
    city_list = fetch_city_list()
    results = [run_speed_test(city_name, threads, multiserver)
        for city_code, city_name in city_list]
    # sorting results based on latency
    results = sorted(results, key=itemgetter('latency'))
    with open('results.json', 'w') as f:
        json.dump(results, f)


@click.command()
@click.option('--subtitle', default=None, help='Optional subtitle for the plot.')
def draw(subtitle):
    """Draw the results graph from results.json file."""
    with open('results.json', 'r') as f:
        results = json.load(f)
    df = process_data(results)
    draw_plot(df, subtitle)


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
