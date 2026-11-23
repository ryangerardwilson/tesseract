import QtQuick

// SpaceCamera is the eye of the space navigator. World space is measured in
// logical pixels of a 1280 by 720 stage, with the nucleus at the origin, X to
// the right, Y up, and Z away from the eye. The workspaces orbit in the
// equatorial plane Y = 0, and the eye looks down on that plane at a shallow
// tilt, so the orbit is a ring seen nearly edge-on that reads as left and
// right. project() is the one pinhole projection for everything: a sphere is
// a disc at its projected place and size, and a ring is a projected polyline,
// so nothing here needs Qt Quick 3D.
QtObject {
    id: lens

    property real width: 1280
    property real height: 720
    // 1 holds the system far from the eye while the camera falls in toward
    // the nucleus on open, 0 at rest.
    property real approach: 0

    readonly property real unit: Math.max(0.5, Math.min(width / 1280, height / 720))
    readonly property real focal: 1700
    readonly property real distance: focal + approach * 2600
    readonly property real tilt: 0.23
    readonly property real centerX: width / 2
    readonly property real centerY: Math.round(height * 0.45)

    // The workspaces: the nucleus, the bodies on the orbit, and how far round
    // the orbit neighbours sit. No body turns past farthestTurn, so lower ids
    // stay on the left and higher ids on the right.
    readonly property real nucleusRadius: 90
    readonly property real bodyRadius: 25
    readonly property real orbitRadius: 530
    readonly property real widestStep: 0.72
    readonly property real farthestTurn: 2.9

    // The shells: the selected layer's shell is the tight one, each deeper
    // shell is one gap wider, and the gaps narrow outward. Shells in front of
    // the selection sink onto the nucleus's skin.
    readonly property real shellRadius: 170
    readonly property real shellGap: 76
    readonly property real narrowing: 0.86
    readonly property real skin: 100
    readonly property real deepestShown: 4
    readonly property real electronRadius: 7
    // Where a layer's electron rests on its shell, by layer index: in pairs
    // to the right and the left, from the front toward the back, so the
    // titles of neighbouring shells never share a place.
    readonly property var homes: [0.87, -0.87, 1.66, -1.66, 2.23, -2.23]
    // How close to the eye the chosen electron rushes on Enter.
    readonly property real rushDepth: 80

    function mix(from, to, amount) {
        return from + (to - from) * amount;
    }

    function smooth(edge0, edge1, value) {
        const t = Math.max(0, Math.min(1, (value - edge0) / (edge1 - edge0)));
        return t * t * (3 - 2 * t);
    }

    // project returns where world point (x, y, z) lands on the stage: its
    // stage pixel, the stage pixels per world unit there, and its depth.
    function project(x, y, z) {
        const lift = z * Math.sin(lens.tilt) + y * Math.cos(lens.tilt);
        const depth = lens.distance + z * Math.cos(lens.tilt) - y * Math.sin(lens.tilt);
        const scale = lens.unit * lens.focal / depth;
        return {
            "x": lens.centerX + x * scale,
            "y": lens.centerY - lift * scale,
            "scale": scale,
            "depth": depth
        };
    }

    // step is the turn between neighbouring workspaces on the orbit.
    function step(count) {
        return Math.min(lens.widestStep, lens.farthestTurn / Math.max(1, count - 1));
    }

    // bodyPose places the workspace `offset` places from the eased selection.
    // At offset 0 it is the nucleus at the centre; from one place out it is a
    // body on the orbit, turned `offset` steps from the front; in between it
    // swings between the two. `spin` turns the whole orbit.
    function bodyPose(offset, step, spin) {
        const turn = offset * step + spin;
        const hold = 1 - lens.smooth(0, 0.9, Math.abs(offset));
        const reach = lens.orbitRadius * (1 - hold);
        const x = reach * Math.sin(turn);
        const z = -reach * Math.cos(turn);
        const at = lens.project(x, 0, z);
        const size = lens.mix(lens.bodyRadius, lens.nucleusRadius, hold);
        const facing = 0.5 + 0.5 * Math.cos(turn);
        return {
            "x": at.x,
            "y": at.y,
            "depth": at.depth,
            "worldX": x,
            "worldZ": z,
            "radius": size * at.scale,
            "size": size / lens.nucleusRadius,
            "hold": hold,
            "light": lens.mix(0.28 + 0.5 * facing, 1, hold),
            // A body the opening spin carries past the back of the orbit is
            // hidden until it comes round, so the ids never wrap.
            "seen": 1 - lens.smooth(2.95, 3.3, Math.abs(turn))
        };
    }

    // shellReach is the radius of the shell `along` layers deeper than the
    // eased selection.
    function shellReach(along) {
        if (along >= 0)
            return lens.shellRadius + lens.shellGap * (1 - Math.pow(lens.narrowing, along)) / (1 - lens.narrowing);

        return lens.skin + (lens.shellRadius - lens.skin) * Math.exp(1.1 * along);
    }

    // shellLight is how bright that shell is: the selected one brightest,
    // deeper ones dimmer and gone past deepestShown, and the ones in front of
    // the selection a faint trace on the nucleus.
    function shellLight(along) {
        if (along < 0)
            return 0.2 + 0.8 * Math.exp(2.4 * along);

        return (1 - lens.smooth(lens.deepestShown, lens.deepestShown + 1, along)) / (1 + 0.85 * along);
    }

    function home(index) {
        return lens.homes[index % lens.homes.length];
    }

    // ring is a whole ring in the equatorial plane as stage points. half
    // selects the half nearer the eye (1), the far half (-1), or all (0).
    function ring(centerX, centerZ, radius, half) {
        const points = [];
        const count = half === 0 ? 96 : 48;
        const start = half < 0 ? Math.PI / 2 : -Math.PI / 2;
        const sweep = half === 0 ? Math.PI * 2 : Math.PI;
        for (let i = 0; i <= count; i++) {
            const angle = start + sweep * i / count;
            const at = lens.project(centerX + radius * Math.sin(angle), 0, centerZ - radius * Math.cos(angle));
            points.push(Qt.point(at.x, at.y));
        }
        return points;
    }

    // electronPose places an electron at `angle` on a shell of `radius`
    // round (centerX, centerZ).
    function electronPose(centerX, centerZ, radius, angle) {
        return lens.project(centerX + radius * Math.sin(angle), 0, centerZ - radius * Math.cos(angle));
    }

    // rushPose carries an electron out of its shell toward the eye: it comes
    // to the middle of the stage as it nears the camera.
    function rushPose(pose, rush) {
        const pull = rush * rush;
        const depth = lens.mix(pose.depth, lens.rushDepth, pull);
        return {
            "x": lens.mix(pose.x, lens.centerX, pull),
            "y": lens.mix(pose.y, lens.height / 2, pull),
            "scale": lens.unit * lens.focal / depth,
            "depth": depth
        };
    }

}
