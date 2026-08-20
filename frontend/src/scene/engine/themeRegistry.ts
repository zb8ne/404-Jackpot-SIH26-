// The slice of the original theme contract the vendored engine actually needs.
// The full registry in the source project also carried cafeteria seats, coffee
// tiles, errand anchors and a TV-show cast — all of which this demo drops.

/** A tile coordinate on the map grid. */
export interface Tile {
  x: number;
  y: number;
}

export type Facing = 'up' | 'down' | 'left' | 'right';

/** Which gids draw a desk monitor, and where the lit overlay sits relative to
 *  the desk's top-left tile. Consumed by DeskScreen. */
export interface MonitorConfig {
  offTopLeftGid: number;
  onGids: Array<[gid: number, dx: number, dy: number]>;
}
