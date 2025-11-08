"""
Leasing - A lightweight distributed lease package for leader election.

This package provides a simple interface for acquiring, renewing, and releasing
distributed leases using pluggable backends (Redis, Kubernetes, File).
"""

from .lease import Lease, LeaseConfig

__all__ = ["Lease", "LeaseConfig"]
__version__ = "0.1.0"
