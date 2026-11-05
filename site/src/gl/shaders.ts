/**
 * The three fragment shaders of §14.4. Each is a single pass behind content.
 * Colours are the tokens: coke #0B0C0E, molten #FF6A1A, ingot #3DDC84, slag #FF4D4D.
 */

const COMMON = `
precision mediump float;
uniform vec2 u_res;
uniform float u_time;
uniform float u_intensity;
const vec3 MOLTEN = vec3(1.0, 0.4157, 0.1020);
float hash(vec2 p){ return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
float vnoise(vec2 p){
  vec2 i = floor(p); vec2 f = fract(p);
  f = f*f*(3.0-2.0*f);
  float a = hash(i), b = hash(i+vec2(1.0,0.0)), c = hash(i+vec2(0.0,1.0)), d = hash(i+vec2(1.0,1.0));
  return mix(mix(a,b,f.x), mix(c,d,f.x), f.y);
}
`;

/**
 * 1. Slag field (hero): 6 px dot grid on coke, radial heat (molten at 6 % peak)
 * following the pointer with lag. The melt-on-exit is gone: in the deck (§0c)
 * the hero card is covered instead of scrolling out, and its field pauses.
 */
export const SLAG_FRAG =
  COMMON +
  `
uniform vec2 u_pointer;   // in pixels, canvas space
uniform float u_dpr;
uniform float u_pitch;    // dot pitch in CSS px: 6, 8.5 on phones (half the dots, §17.2)
uniform float u_dot;      // dot brightness: 0.055 hero, 0.03 discreet
uniform float u_heat;     // heat peak on the ground: 0.06 hero, 0.02 discreet
uniform float u_radius;   // heat radius in px: 420 hero, 252 discreet (60 %)
void main(){
  vec2 px = gl_FragCoord.xy;
  float pitch = u_pitch * u_dpr;
  vec2 cell = mod(px, pitch) - pitch * 0.5;
  float dot = 1.0 - smoothstep(0.6 * u_dpr, 1.1 * u_dpr, length(cell));
  float d = distance(px, u_pointer) / (u_radius * u_dpr);
  float heat = exp(-d * d * 4.0);
  float a = dot * (u_dot + heat * 0.35) * u_intensity;
  vec3 col = mix(vec3(0.25, 0.26, 0.29), MOLTEN, heat * 0.9);
  float glow = heat * u_heat * u_intensity;
  gl_FragColor = vec4(col * a + MOLTEN * glow, a + glow);
}
`;

/**
 * 2. Furnace (the pin): value-noise ember glow from the top edge, drifting at
 * 0.02 Hz, amplitude by depth (0.3 on K1 → 1.0 on K4). Hue is fixed molten;
 * only luminance moves. Peak luminance on the ground stays under 12 %.
 * Scanline: 1 px darkening every 3 px at 4 %, static.
 */
export const FURNACE_FRAG =
  COMMON +
  `
uniform float u_dpr;
void main(){
  vec2 uv = gl_FragCoord.xy / u_res;
  float fromTop = 1.0 - uv.y;
  float drift = u_time * 0.02 * 6.2831;
  float n = vnoise(vec2(uv.x * 3.0 + sin(drift) * 0.5, fromTop * 2.0 + drift * 0.15));
  n = 0.6 + 0.4 * n;
  float fall = exp(-fromTop * 3.2);
  float lum = fall * n * 0.12 * u_intensity;             // ≤ 12 %
  float scan = step(mod(gl_FragCoord.y / u_dpr, 3.0), 1.0) * 0.04;
  vec3 col = MOLTEN * lum;
  // darken on scanline rows: premultiplied black at 4 %
  gl_FragColor = vec4(col * (1.0 - scan), lum + scan);
}
`;

/**
 * 3. Verdict flash (terminals): one horizontal band of +3 % luminance sweeps
 * top to bottom over 240 ms, ingot-tinted for exit 0, slag otherwise.
 * u_sweep ∈ [0, 1] is the band position; u_tint 1 = ingot, 0 = slag.
 */
export const FLASH_FRAG =
  COMMON +
  `
uniform float u_sweep;
uniform float u_tint;
void main(){
  vec2 uv = gl_FragCoord.xy / u_res;
  float y = 1.0 - uv.y;
  float band = exp(-pow((y - u_sweep) / 0.08, 2.0));
  vec3 ingot = vec3(0.2392, 0.8627, 0.5176);
  vec3 slag = vec3(1.0, 0.3020, 0.3020);
  vec3 col = mix(slag, ingot, u_tint);
  float a = band * 0.03 * u_intensity * 3.0;
  gl_FragColor = vec4(col * a, a);
}
`;
