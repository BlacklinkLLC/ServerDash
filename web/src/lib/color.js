function hexToRgb(hex) {
	const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex);
	if (!m) return { r: 255, g: 176, b: 32 }; // falls back to NOVA's amber
	return { r: parseInt(m[1], 16), g: parseInt(m[2], 16), b: parseInt(m[3], 16) };
}

function toHex(n) {
	return Math.max(0, Math.min(255, Math.round(n)))
		.toString(16)
		.padStart(2, '0');
}

/** Applies a custom accent color across every --accent-* token the design
 * system derives from it, not just the base color, so a custom brand color
 * looks intentional (soft washes, hover states) rather than half-themed. */
export function applyAccentColor(hex) {
	const { r, g, b } = hexToRgb(hex);
	const root = document.documentElement.style;
	root.setProperty('--accent', hex);
	root.setProperty('--accent-d', `#${toHex(r * 0.8)}${toHex(g * 0.8)}${toHex(b * 0.8)}`);
	root.setProperty('--accent-soft', `rgba(${r}, ${g}, ${b}, 0.12)`);
	root.setProperty('--accent-glow', `rgba(${r}, ${g}, ${b}, 0.08)`);
	root.setProperty('--border-strong', `rgba(${r}, ${g}, ${b}, 0.28)`);
}
