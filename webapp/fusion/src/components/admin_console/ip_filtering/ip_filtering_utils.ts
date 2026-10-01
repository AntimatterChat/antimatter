// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import ipaddr from 'ipaddr.js';

import type {AllowedIPRange} from '@mattermost/types/config';

type Range = [ipaddr.IPv4 | ipaddr.IPv6, number];

function normalize(addr: ipaddr.IPv4 | ipaddr.IPv6): ipaddr.IPv4 | ipaddr.IPv6 {
    if (addr.kind() === 'ipv6' && (addr as ipaddr.IPv6).isIPv4MappedAddress()) {
        return (addr as ipaddr.IPv6).toIPv4Address();
    }
    return addr;
}

// parseRange accepts a range in CIDR notation or a single address, as the server does.
// IPv4-mapped IPv6 ranges are turned into their IPv4 form.
export function parseRange(block: string): Range {
    const value = block.trim();
    if (!value.includes('/')) {
        const addr = normalize(ipaddr.parse(value));
        return [addr, addr.kind() === 'ipv4' ? 32 : 128];
    }

    const [addr, bits] = ipaddr.parseCIDR(value);
    if (addr.kind() === 'ipv6' && (addr as ipaddr.IPv6).isIPv4MappedAddress()) {
        if (bits < 96) {
            throw new Error('range covers more than the IPv4-mapped space');
        }
        return [(addr as ipaddr.IPv6).toIPv4Address(), bits - 96];
    }
    return [addr, bits];
}

export function isDenyRule(range: AllowedIPRange): boolean {
    return range.action === 'deny';
}

export function isIPAddressInRanges(ipAddress: string, ranges: AllowedIPRange[]): boolean {
    const usersAddr = normalize(ipaddr.parse(ipAddress));

    for (const range of ranges) {
        let parsed: Range;
        try {
            parsed = parseRange(range.cidr_block);
        } catch {
            continue;
        }

        if (usersAddr.kind() !== parsed[0].kind()) {
            continue;
        }

        if (usersAddr.match(parsed)) {
            return true;
        }
    }

    return false;
}

// isIPAddressAllowed mirrors the server: an address matching an enabled deny rule is
// refused, and if any allow rule is enabled the address must match one of them.
export function isIPAddressAllowed(ipAddress: string, ranges: AllowedIPRange[]): boolean {
    const enabled = ranges.filter((range) => range.enabled);
    const deny = enabled.filter(isDenyRule);
    const allow = enabled.filter((range) => !isDenyRule(range));

    if (!deny.length && !allow.length) {
        return true;
    }

    try {
        if (isIPAddressInRanges(ipAddress, deny)) {
            return false;
        }
        return !allow.length || isIPAddressInRanges(ipAddress, allow);
    } catch {
        return false;
    }
}

export function validateCIDR(cidr: string) {
    try {
        parseRange(cidr);
    } catch {
        return false;
    }

    return true;
}
