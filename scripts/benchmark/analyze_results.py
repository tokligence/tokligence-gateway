#!/usr/bin/env python3
"""
Analyze benchmark results and generate comparison charts.

Usage:
    python analyze_results.py results_stats.csv
    python analyze_results.py results_stats.csv --compare litellm
"""

import argparse
import csv
import json
import sys
from pathlib import Path
from typing import Dict, List, Tuple

try:
    import pandas as pd
    import matplotlib.pyplot as plt
    import matplotlib
    matplotlib.use('Agg')  # Non-interactive backend
except ImportError:
    print("Error: Required packages not installed.")
    print("Install with: pip install pandas matplotlib")
    sys.exit(1)


# LiteLLM benchmark results (4 CPU, 8GB RAM, 4 instances)
LITELLM_BENCHMARK = {
    "rps": 1170,
    "median_latency": 100,
    "p95_latency": 150,
    "p99_latency": 240,
    "avg_latency": 111.73,
    "overhead": (2, 8),  # min-max overhead in ms
}


def parse_locust_csv(csv_path: Path) -> Dict:
    """Parse Locust stats CSV file."""
    stats = {}

    with open(csv_path, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            if row['Name'] == 'Aggregated':
                stats = {
                    'total_requests': int(row['Request Count']),
                    'total_failures': int(row['Failure Count']),
                    'rps': float(row['Requests/s']),
                    'median_latency': float(row['Median Response Time']),
                    'avg_latency': float(row['Average Response Time']),
                    'min_latency': float(row['Min Response Time']),
                    'max_latency': float(row['Max Response Time']),
                }

                # P95 and P99 if available
                if '95%' in row and row['95%']:
                    stats['p95_latency'] = float(row['95%'])
                if '99%' in row and row['99%']:
                    stats['p99_latency'] = float(row['99%'])

                # Error rate
                if stats['total_requests'] > 0:
                    stats['error_rate'] = (stats['total_failures'] / stats['total_requests']) * 100
                else:
                    stats['error_rate'] = 0

                break

    return stats


def generate_comparison_table(tokligence_stats: Dict, litellm_stats: Dict = None):
    """Generate comparison table."""
    if litellm_stats is None:
        litellm_stats = LITELLM_BENCHMARK

    print("\n" + "="*70)
    print("Tokligence Gateway vs LiteLLM Benchmark Comparison")
    print("="*70)
    print(f"{'Metric':<25} {'Tokligence':<15} {'LiteLLM':<15} {'Status':<15}")
    print("-"*70)

    # RPS
    tok_rps = tokligence_stats['rps']
    lit_rps = litellm_stats['rps']
    rps_diff = ((tok_rps - lit_rps) / lit_rps) * 100
    rps_status = "✅ Good" if tok_rps >= lit_rps * 0.85 else "⚠️ Below"
    print(f"{'RPS':<25} {tok_rps:<15.1f} {lit_rps:<15.1f} {rps_status:<15}")

    # Median Latency
    tok_med = tokligence_stats['median_latency']
    lit_med = litellm_stats['median_latency']
    med_diff = ((tok_med - lit_med) / lit_med) * 100
    med_status = "✅ Good" if tok_med <= lit_med * 1.2 else "⚠️ High"
    print(f"{'Median Latency (ms)':<25} {tok_med:<15.1f} {lit_med:<15.1f} {med_status:<15}")

    # P95 Latency
    if 'p95_latency' in tokligence_stats:
        tok_p95 = tokligence_stats['p95_latency']
        lit_p95 = litellm_stats['p95_latency']
        p95_diff = ((tok_p95 - lit_p95) / lit_p95) * 100
        p95_status = "✅ Good" if tok_p95 <= lit_p95 * 1.2 else "⚠️ High"
        print(f"{'P95 Latency (ms)':<25} {tok_p95:<15.1f} {lit_p95:<15.1f} {p95_status:<15}")

    # P99 Latency
    if 'p99_latency' in tokligence_stats:
        tok_p99 = tokligence_stats['p99_latency']
        lit_p99 = litellm_stats['p99_latency']
        p99_diff = ((tok_p99 - lit_p99) / lit_p99) * 100
        p99_status = "✅ Good" if tok_p99 <= lit_p99 * 1.25 else "⚠️ High"
        print(f"{'P99 Latency (ms)':<25} {tok_p99:<15.1f} {lit_p99:<15.1f} {p99_status:<15}")

    # Error Rate
    tok_err = tokligence_stats['error_rate']
    err_status = "✅ Good" if tok_err < 1.0 else "❌ High"
    print(f"{'Error Rate (%)':<25} {tok_err:<15.2f} {'< 1.0':<15} {err_status:<15}")

    print("-"*70)
    print(f"\nNote: LiteLLM used 4 instances, Tokligence uses 1 instance")
    print("="*70)


def generate_charts(tokligence_stats: Dict, output_dir: Path):
    """Generate comparison charts."""
    output_dir.mkdir(exist_ok=True)

    # Chart 1: Latency comparison
    fig, ax = plt.subplots(figsize=(10, 6))

    metrics = []
    tokligence_values = []
    litellm_values = []

    if 'median_latency' in tokligence_stats:
        metrics.append('Median')
        tokligence_values.append(tokligence_stats['median_latency'])
        litellm_values.append(LITELLM_BENCHMARK['median_latency'])

    if 'p95_latency' in tokligence_stats:
        metrics.append('P95')
        tokligence_values.append(tokligence_stats['p95_latency'])
        litellm_values.append(LITELLM_BENCHMARK['p95_latency'])

    if 'p99_latency' in tokligence_stats:
        metrics.append('P99')
        tokligence_values.append(tokligence_stats['p99_latency'])
        litellm_values.append(LITELLM_BENCHMARK['p99_latency'])

    x = range(len(metrics))
    width = 0.35

    ax.bar([i - width/2 for i in x], tokligence_values, width, label='Tokligence')
    ax.bar([i + width/2 for i in x], litellm_values, width, label='LiteLLM')

    ax.set_ylabel('Latency (ms)')
    ax.set_title('Latency Comparison: Tokligence vs LiteLLM')
    ax.set_xticks(x)
    ax.set_xticklabels(metrics)
    ax.legend()
    ax.grid(axis='y', alpha=0.3)

    plt.tight_layout()
    plt.savefig(output_dir / 'latency_comparison.png', dpi=150)
    print(f"✓ Generated: {output_dir / 'latency_comparison.png'}")

    # Chart 2: Throughput comparison
    fig, ax = plt.subplots(figsize=(8, 6))

    categories = ['Tokligence\n(1 instance)', 'LiteLLM\n(4 instances)']
    rps_values = [tokligence_stats['rps'], LITELLM_BENCHMARK['rps']]
    colors = ['#2ecc71', '#3498db']

    bars = ax.bar(categories, rps_values, color=colors, alpha=0.7)
    ax.set_ylabel('Requests per Second')
    ax.set_title('Throughput Comparison')
    ax.grid(axis='y', alpha=0.3)

    # Add value labels on bars
    for bar, val in zip(bars, rps_values):
        height = bar.get_height()
        ax.text(bar.get_x() + bar.get_width()/2., height,
                f'{val:.0f} RPS',
                ha='center', va='bottom')

    plt.tight_layout()
    plt.savefig(output_dir / 'throughput_comparison.png', dpi=150)
    print(f"✓ Generated: {output_dir / 'throughput_comparison.png'}")

    plt.close('all')


def main():
    parser = argparse.ArgumentParser(description='Analyze benchmark results')
    parser.add_argument('csv_file', help='Locust stats CSV file')
    parser.add_argument('--compare', choices=['litellm'], default='litellm',
                        help='Compare with benchmark')
    parser.add_argument('--output-dir', default='charts',
                        help='Output directory for charts')

    args = parser.parse_args()

    csv_path = Path(args.csv_file)
    if not csv_path.exists():
        print(f"Error: File not found: {csv_path}")
        sys.exit(1)

    # Parse results
    print(f"Parsing results from: {csv_path}")
    tokligence_stats = parse_locust_csv(csv_path)

    if not tokligence_stats:
        print("Error: No statistics found in CSV")
        sys.exit(1)

    # Display raw stats
    print("\n" + "="*70)
    print("Tokligence Gateway Benchmark Results")
    print("="*70)
    print(f"Total Requests:      {tokligence_stats['total_requests']:,}")
    print(f"Total Failures:      {tokligence_stats['total_failures']:,}")
    print(f"Requests/sec:        {tokligence_stats['rps']:.2f}")
    print(f"Median Latency:      {tokligence_stats['median_latency']:.1f} ms")
    print(f"Average Latency:     {tokligence_stats['avg_latency']:.2f} ms")
    print(f"Min Latency:         {tokligence_stats['min_latency']:.1f} ms")
    print(f"Max Latency:         {tokligence_stats['max_latency']:.1f} ms")

    if 'p95_latency' in tokligence_stats:
        print(f"P95 Latency:         {tokligence_stats['p95_latency']:.1f} ms")
    if 'p99_latency' in tokligence_stats:
        print(f"P99 Latency:         {tokligence_stats['p99_latency']:.1f} ms")

    print(f"Error Rate:          {tokligence_stats['error_rate']:.2f}%")
    print("="*70)

    # Generate comparison
    if args.compare == 'litellm':
        generate_comparison_table(tokligence_stats)

    # Generate charts
    output_dir = Path(args.output_dir)
    generate_charts(tokligence_stats, output_dir)

    print(f"\n✓ Analysis complete!")


if __name__ == '__main__':
    main()
