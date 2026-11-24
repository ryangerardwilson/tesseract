pragma ComponentBehavior: Bound
import QtQuick

// SpaceStage is the navigator's picture: a black void, the selected
// workspace as a glowing nucleus at the centre, the other workspaces as
// smaller bodies on one equatorial orbit around it, and the nucleus's windows
// as electrons on concentric shells. It only draws; SpaceNavigator owns the
// data, the keys, the carried window, and the focus and move commands.
Item {
    id: stage

    // How dark the void behind the system is. A host with its own backdrop
    // clears it.
    property real voidOpacity: 0.97
    property var workspaces: []
    property int activeId: 0
    property int column: 0
    property int chosenLayer: 0
    // The moves on each axis counted without wrapping: a cycle past an end
    // still turns the picture one step.
    property int columnSteps: 0
    property int layerSteps: 0
    property bool loading: true
    // A window is carried. It is the selected layer, and it keeps the tight
    // shell, brighter, while the other layers ease past it.
    property bool carrying: false
    // The carried layout holds until the picture rests after a carry ends,
    // so a cancel eases back instead of jumping.
    property bool pinned: false
    // How bright the carried electron is, from 0 to 1.
    property real lift: carrying ? 1 : 0
    // How far the orbit has turned, in workspaces, without wrapping. It eases
    // to `columnSteps`, and the workspace it is nearest holds the nucleus.
    property real orbit: 0
    // An extra turn of the whole orbit, in radians, that settles on open.
    property real spin: 0
    // 0 while the shells are folded into the nucleus, 1 when they are out.
    property real unfurl: 0
    property real presence: 0
    // The eased layer of the selected workspace, and the layer the workspace
    // leaving the nucleus held when the selection moved off it, both counted
    // without wrapping.
    property real depth: 0
    property real leavingDepth: 0
    property int heldColumn: 0
    property real rush: 0
    // The chosen electron's idle swing along its shell, from -1 to 1.
    property real drift: 0
    property real breath: 0
    // How brightly the nucleus blooms, from 0 to 1: once as the shells open,
    // and briefly as each move settles.
    property real bloom: 0
    // The light is white, a little warmer toward the core and cooler toward
    // the rim.
    readonly property color warm: Qt.rgba(1, 0.955, 0.9, 1)
    readonly property color cool: Qt.rgba(0.87, 0.925, 1, 1)
    readonly property real step: eye.step(workspaces.length)
    readonly property int owner: workspaces.length > 0 ? stage.place(Math.round(orbit), workspaces.length) : -1
    readonly property real ownerOffset: Math.round(orbit) - orbit
    readonly property var ownerLayers: owner >= 0 ? (workspaces[owner].layers || []) : []
    readonly property real ownerDepth: owner === column ? depth : leavingDepth
    readonly property var orbitLaps: laps(orbit, workspaces.length)
    readonly property var nucleus: eye.bodyPose(ownerOffset, step, spin)
    readonly property real shellsOut: unfurl * (1 - eye.smooth(0.06, 0.42, Math.abs(ownerOffset)))
    readonly property var spots: layerSpots()
    readonly property var shells: shellPoses()
    readonly property var electrons: electronPoses()
    readonly property var dust: dustPoses()
    readonly property var selectedWorkspace: column < workspaces.length ? workspaces[column] : null
    readonly property int layerCount: selectedWorkspace ? (selectedWorkspace.layers || []).length : 0
    readonly property real rushFade: 1 - eye.smooth(0, 0.45, rush)

    // The chosen electron has reached the eye.
    signal rushed()

    // arrive opens the system: the camera falls in toward the nucleus while
    // the orbit settles out of a slow spin onto the selected workspace, and
    // then the shells unfurl.
    function arrive() {
        sink.stop();
        orbitEase.enabled = false;
        stage.orbit = stage.columnSteps;
        orbitEase.enabled = true;
        stage.heldColumn = stage.column;
        stage.depth = stage.layerSteps;
        stage.leavingDepth = stage.layerSteps;
        opening.restart();
        // The nucleus blooms as its shells start to open.
        stage.bloom = 0;
        stage.bloomAt(420, 1, 240, 1100);
    }

    // bloomAt blooms the nucleus from wherever its bloom stands: after
    // `delay` ms it rises to `peak`, or holds a brighter bloom, for `rise`
    // ms, then fades for `fall` ms. It never holds a key back.
    function bloomAt(delay, peak, rise, fall) {
        blooming.stop();
        bloomDelay.duration = delay;
        bloomRise.to = Math.max(peak, stage.bloom);
        bloomRise.duration = rise;
        bloomFall.duration = fall;
        blooming.start();
    }

    // place is the index an unbounded position lands on among `count`.
    function place(position, count) {
        return ((position % count) + count) % count;
    }

    // laps splits an unbounded position on a cycle of `count` places into the
    // two laps of that cycle that can show: `base` is where the lap the
    // position is in starts, and `here` and `next` are how much of that lap
    // and of the one after it show. Between the last place of a lap and the
    // first of the next, both laps roll one step on while they crossfade, so
    // a cycle past an end turns one step and never the long way round.
    function laps(position, count) {
        const span = Math.max(1, count);
        const lap = Math.floor(position / span);
        const roll = Math.max(0, position - lap * span - (span - 1));
        return {
            "base": lap * span,
            "here": 1 - eye.smooth(0.3, 1, roll),
            "next": eye.smooth(0, 0.7, roll)
        };
    }

    // rushIn carries the chosen electron into the eye, then emits rushed().
    function rushIn() {
        rushAnimation.restart();
    }

    function headline() {
        if (stage.selectedWorkspace) {
            const name = "WORKSPACE " + String(stage.selectedWorkspace.name || stage.selectedWorkspace.id).toUpperCase();
            if (stage.layerCount === 0)
                return name + "   ·   NO APPS";

            return name + "   ·   APP " + (stage.chosenLayer + 1) + " / " + stage.layerCount;
        }
        return stage.loading ? "READING HYPRLAND" : "NO WORKSPACES";
    }

    // layerSpots says where each of the nucleus's layers stands, once for
    // each of the two laps of layers that can show: `along` is how many
    // layers deeper than the eased selection it is, `home` where its
    // electron rests round its shell, `light`, `weight`, and `glow` how its
    // ring and electron are lit, and `shell` and `lap` how much of its ring
    // and electron show.
    function layerSpots() {
        const spots = [];
        const count = stage.ownerLayers.length;
        const depth = stage.ownerDepth;
        const laps = stage.laps(depth, count);
        const carry = stage.pinned && stage.owner === stage.column;
        for (let index = 0; index < count * 2; index++) {
            if (carry) {
                spots.push(stage.carrySpot(index, count, depth, laps));
                continue;
            }
            const along = laps.base + index - depth;
            const lap = index < count ? laps.here : laps.next;
            spots.push({
                "along": along,
                "home": eye.home(index % count),
                "chosen": Math.max(0, 1 - Math.abs(along)),
                "carried": false,
                "light": eye.shellLight(along),
                "weight": 1 + 0.6 * Math.max(0, 1 - Math.abs(along)),
                "glow": eye.shellLight(Math.max(0, along)),
                "shell": lap,
                "lap": lap
            });
        }
        return spots;
    }

    // carrySpot places one layer while a window is carried. The carried
    // window stays on the tight shell, and its two electrons cross-fade from
    // the resting place of the slot it leaves to that of the slot it takes,
    // as the selection does when it moves. Every other layer keeps its order
    // and stands one slot further out until the carried window passes in
    // front of it; its electron hides while it passes.
    function carrySpot(index, count, depth, laps) {
        const slot = index % count;
        const carried = Math.min(stage.chosenLayer, count - 1);
        const second = index >= count;
        if (slot === carried) {
            const from = Math.floor(depth);
            const blend = depth - from;
            return {
                "along": 0,
                "home": eye.home(stage.place(second ? from + 1 : from, count)),
                "chosen": 1,
                "carried": true,
                "light": 1,
                "weight": 1.6,
                "glow": 1,
                "shell": second ? 0 : 1,
                "lap": second ? blend : 1 - blend
            };
        }
        const order = slot < carried ? slot : slot - 1;
        const reach = depth - laps.base - (second ? count : 0);
        const seat = order + Math.max(0, Math.min(1, order + 1 - reach));
        const along = seat - reach;
        const lap = second ? laps.next : laps.here;
        const passing = Math.abs(seat - Math.round(seat));
        return {
            "along": along,
            "home": eye.home(Math.round(seat)),
            "chosen": 0,
            "carried": false,
            "light": Math.min(eye.shellLight(along), eye.shellLight(1)),
            "weight": 1,
            "glow": Math.min(eye.shellLight(Math.max(0, along)), eye.shellLight(1)),
            "shell": lap,
            "lap": lap * (1 - eye.smooth(0.2, 0.45, passing))
        };
    }

    // shellPoses lays out the nucleus's shells: each layer's ring, split
    // into the half behind the nucleus and the half in front of it, once for
    // each of the two laps of layers that can show.
    function shellPoses() {
        const poses = [];
        const out = stage.shellsOut;
        const center = stage.nucleus;
        for (const spot of stage.spots) {
            const light = spot.light * out * stage.rushFade * spot.shell;
            const reach = eye.mix(eye.skin, eye.shellReach(spot.along), out) * center.size;
            const shown = light > 0.004;
            poses.push({
                "along": spot.along,
                "light": light,
                "weight": spot.weight,
                "tint": stage.shade(spot.along),
                "back": shown ? eye.ring(center.worldX, center.worldZ, reach, -1) : [],
                "front": shown ? eye.ring(center.worldX, center.worldZ, reach, 1) : []
            });
        }
        return poses;
    }

    // shimmer is how brightly the soft band of the shell `along` layers
    // deeper than the selection shines: each breath of the halo carries the
    // light out through the shells and back in.
    function shimmer(along) {
        const lead = 0.13 * Math.max(0, along);
        return 0.3 + 0.7 * eye.smooth(lead, lead + 0.48, stage.breath);
    }

    // shade is the tint of the shell `along` layers deeper than the
    // selection: warmer as it sinks onto the core, cooler further out.
    function shade(along) {
        const warmth = eye.smooth(0, 1.5, -along);
        const chill = eye.smooth(0, 3.5, along);
        const warm = stage.warm;
        const cool = stage.cool;
        return Qt.rgba(1 - (1 - warm.r) * warmth - (1 - cool.r) * chill, 1 - (1 - warm.g) * warmth - (1 - cool.g) * chill, 1 - (1 - warm.b) * warmth - (1 - cool.b) * chill, 1);
    }

    // electronPoses places each layer's electron on its shell. The chosen one
    // drifts a little along its shell while the picture rests, trailing a
    // little light along it, and rushes toward the eye on Enter, leaving a
    // streak. Like the shells, each layer has one electron in each of the two
    // laps.
    function electronPoses() {
        const poses = [];
        const out = stage.shellsOut;
        const center = stage.nucleus;
        const turn = stage.ownerOffset * stage.step - (1 - out) * 1.7;
        const count = stage.ownerLayers.length;
        const trailing = 1 - eye.smooth(0, 0.3, stage.rush);
        for (let index = 0; index < stage.spots.length; index++) {
            const spot = stage.spots[index];
            const along = spot.along;
            const chosen = spot.chosen;
            const reach = eye.mix(eye.skin, eye.shellReach(along), out) * center.size;
            const home = spot.home;
            const side = home >= 0 ? 1 : -1;
            const angle = home + turn + chosen * stage.drift * 0.13;
            const rest = eye.electronPose(center.worldX, center.worldZ, reach, angle);
            const rushing = stage.owner === stage.column && index % count === stage.chosenLayer;
            const at = rushing ? eye.rushPose(rest, stage.rush) : rest;
            const fade = along < 0 ? 1 - eye.smooth(0, 0.55, -along) : 1 - eye.smooth(eye.deepestShown, eye.deepestShown + 1, along);
            const lift = spot.carried ? stage.lift : 0;
            const trail = chosen * trailing;
            const streaking = rushing && stage.rush > 0;
            poses.push({
                "x": at.x,
                "y": at.y,
                "depth": at.depth,
                "radius": eye.electronRadius * (1 + 0.45 * chosen + 0.3 * lift) * at.scale * center.size,
                "chosen": chosen,
                "carried": lift,
                "light": spot.glow,
                "side": side,
                "grow": at.scale / rest.scale,
                "shown": fade * eye.smooth(0.45, 1, out) * (rushing ? 1 - eye.smooth(0.8, 1, stage.rush) : stage.rushFade) * spot.lap,
                "trail": trail > 0.01 ? stage.trailPath(center, reach, angle, side, at) : [],
                "trailLight": trail,
                "streak": streaking ? stage.streakPath(rest, at) : [],
                "streakLight": streaking ? eye.smooth(0, 0.12, stage.rush) : 0
            });
        }
        return poses;
    }

    // trailPath is the chosen electron's trail: points along its shell from
    // the electron back toward the front of the shell, away from its title,
    // relative to where the electron is drawn. It stretches a little as the
    // electron swings out.
    function trailPath(center, reach, angle, side, at) {
        const length = eye.trailLength * (1 + 0.25 * side * stage.drift);
        return eye.arc(center.worldX, center.worldZ, reach, angle, angle - side * length, 10).map((point) => {
            return Qt.point(point.x - at.x, point.y - at.y);
        });
    }

    // streakPath is the rushing electron's streak: points from where it is
    // back to where it was a moment before, relative to where it is.
    function streakPath(rest, at) {
        const tail = eye.rushPose(rest, Math.max(0, stage.rush - 0.55));
        const points = [];
        for (let i = 0; i <= 6; i++)
            points.push(Qt.point((tail.x - at.x) * i / 6, (tail.y - at.y) * i / 6));
        return points;
    }

    // dustPoses places the orbit's dust. Every mote turns with the orbit and
    // the opening spin, so the dust travels with the workspaces, and the
    // motes are brighter and larger toward the front.
    function dustPoses() {
        const poses = [];
        const turn = stage.spin - stage.orbit * stage.step;
        for (let index = 0; index < eye.moteCount; index++) {
            const mote = eye.mote(index);
            const angle = mote.angle + turn;
            const at = eye.project(mote.reach * Math.sin(angle), mote.height, -mote.reach * Math.cos(angle));
            const facing = 0.5 + 0.5 * Math.cos(angle);
            poses.push({
                "x": at.x,
                "y": at.y,
                "depth": at.depth,
                "size": mote.size * 2.4 * at.scale,
                "light": 0.1 + 0.9 * facing * facing
            });
        }
        return poses;
    }

    onColumnChanged: {
        // The workspace in the nucleus keeps the layer it shows until the
        // orbit carries it away; a workspace coming back picks up from there.
        // Its layers repeat every lap, so it comes back by whole laps to the
        // lap of its front layer, where the selection restarts.
        const shown = stage.owner === stage.heldColumn ? stage.depth : stage.leavingDepth;
        sink.stop();
        if (stage.owner === stage.column) {
            const count = Math.max(1, stage.ownerLayers.length);
            stage.depth = shown - count * Math.round(shown / count);
            sink.to = stage.layerSteps;
            sink.restart();
        } else {
            // A carried window leaves with the selection, and the workspace
            // it leaves folds with the gap it left at the tight shell.
            stage.leavingDepth = stage.carrying ? stage.chosenLayer - 0.5 : shown;
            stage.depth = stage.layerSteps;
        }
        stage.heldColumn = stage.column;
        stage.orbit = stage.columnSteps;
        stage.bloomAt(0, 0.6, 260, 700);
    }
    onCarryingChanged: {
        if (stage.carrying)
            stage.pinned = true;
        else if (!sink.running)
            stage.pinned = false;
    }
    onLayerStepsChanged: {
        if (stage.owner === stage.column) {
            sink.to = stage.layerSteps;
            sink.restart();
            stage.bloomAt(0, 0.4, 220, 560);
        } else {
            sink.stop();
            stage.depth = stage.layerSteps;
        }
    }

    SpaceCamera {
        id: eye

        width: stage.width
        height: stage.height
    }

    Rectangle {
        anchors.fill: parent
        color: "#020203"
        opacity: stage.voidOpacity * (1 - eye.smooth(0.55, 1, stage.rush))
    }

    Item {
        anchors.fill: parent
        opacity: stage.presence

        SpaceGlow {
            x: eye.centerX
            y: eye.centerY
            z: -10000000
            radius: eye.nucleusRadius * (4.6 + 0.8 * stage.bloom) * eye.unit * eye.focal / eye.distance
            strength: (0.07 + 0.025 * stage.breath + 0.03 * stage.bloom) * stage.rushFade
            color: stage.warm
        }

        SpaceRing {
            z: -9999999
            lens: eye
            points: eye.ring(0, 0, eye.orbitRadius, 0)
            light: 0.6 * stage.rushFade
            color: stage.cool
        }

        // The orbit's dust: a thin band of motes that turns with the orbit,
        // brighter toward the front and lit a little by the halo's breath.
        Repeater {
            model: eye.moteCount

            Item {
                id: speck

                required property int index
                readonly property var mote: stage.dust[index] || null

                x: mote ? mote.x : 0
                y: mote ? mote.y : 0
                z: mote ? -mote.depth : 0
                opacity: mote ? mote.light * (0.7 + 0.3 * stage.breath) * stage.rushFade : 0
                visible: opacity > 0.01

                Rectangle {
                    readonly property real size: speck.mote ? speck.mote.size * 3.4 : 0

                    x: -size / 2
                    y: -size / 2
                    width: size
                    height: size
                    radius: size / 2
                    color: stage.cool
                    opacity: 0.12
                }

                Rectangle {
                    readonly property real size: speck.mote ? speck.mote.size : 0

                    x: -size / 2
                    y: -size / 2
                    width: size
                    height: size
                    radius: size / 2
                    antialiasing: true
                    color: stage.cool
                    opacity: 0.9
                }

            }

        }

        // Each workspace has one body in each of the two laps of the orbit.
        Repeater {
            model: stage.workspaces.length * 2

            SpaceBody {
                required property int index

                space: stage.workspaces[index % stage.workspaces.length]
                pose: eye.bodyPose(stage.orbitLaps.base + index - stage.orbit, stage.step, stage.spin)
                active: space.id === stage.activeId
                breath: stage.breath
                bloom: stage.bloom
                sway: stage.drift
                warm: stage.warm
                cool: stage.cool
                opacity: stage.rushFade * (index < stage.workspaces.length ? stage.orbitLaps.here : stage.orbitLaps.next)
                visible: opacity > 0.004
            }

        }

        // Each shell is a ring, the half behind the nucleus and the half in
        // front, over a soft band that shimmers as the halo breathes.
        Repeater {
            model: stage.ownerLayers.length * 2

            SpaceRing {
                required property int index
                readonly property var shell: stage.shells[index] || null

                z: -stage.nucleus.depth - 0.5
                lens: eye
                points: shell ? shell.back : []
                light: shell ? shell.light * (1 + 0.5 * stage.bloom) : 0
                weight: shell ? shell.weight : 1
                shimmer: shell ? stage.shimmer(shell.along) : 0
                color: shell ? shell.tint : "white"
            }

        }

        Repeater {
            model: stage.ownerLayers.length * 2

            SpaceRing {
                required property int index
                readonly property var shell: stage.shells[index] || null

                z: -stage.nucleus.depth + 0.5
                lens: eye
                points: shell ? shell.front : []
                light: shell ? shell.light * (1 + 0.5 * stage.bloom) : 0
                weight: shell ? shell.weight : 1
                shimmer: shell ? stage.shimmer(shell.along) : 0
                color: shell ? shell.tint : "white"
            }

        }

        Repeater {
            model: stage.ownerLayers.length * 2

            SpaceSatellite {
                required property int index
                readonly property int layerIndex: index % stage.ownerLayers.length
                readonly property var client: stage.ownerLayers[layerIndex] || ({
                })

                z: 10000000 + (layerIndex === stage.chosenLayer ? 1 : 0)
                lens: eye
                pose: stage.electrons[index] || ({
                })
                windowClass: String(client["class"] || "")
                windowTitle: String(client.title || "")
                breath: stage.breath
                cool: stage.cool
            }

        }

    }

    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Math.round(64 * eye.unit)
        text: stage.headline()
        textFormat: Text.PlainText
        color: "#8d8d8d"
        opacity: stage.rushFade
        font.family: "JetBrainsMono Nerd Font"
        font.pixelSize: Math.round(10 * eye.unit)
        font.weight: Font.Bold
        font.letterSpacing: 1.5 * eye.unit
    }

    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Math.round(24 * eye.unit)
        text: stage.carrying ? "SHIFT      H L  WORKSPACE      K J  APP      S  PUT DOWN      Q  CANCEL" : "H L  WORKSPACE      K J  APP      ENTER  FOCUS      N  NEW      DD  CLOSE      Q  CLOSE"
        textFormat: Text.PlainText
        color: stage.carrying ? "#bdbdbd" : "#5c5c5c"
        opacity: stage.rushFade
        font.family: "JetBrainsMono Nerd Font"
        font.pixelSize: Math.round(9 * eye.unit)
        font.weight: Font.Bold
    }

    // The orbit turns with an ease in and out, and a turn that is retargeted
    // mid-way keeps its speed.
    Behavior on orbit {
        id: orbitEase

        SmoothedAnimation {
            duration: 400
            velocity: -1
        }

    }

    Behavior on lift {
        NumberAnimation {
            duration: 220
            easing.type: Easing.OutCubic
        }

    }

    NumberAnimation {
        id: sink

        target: stage
        property: "depth"
        duration: 380
        easing.type: Easing.OutCubic
        onStopped: {
            if (!stage.carrying)
                stage.pinned = false;

        }
    }

    ParallelAnimation {
        id: opening

        NumberAnimation {
            target: stage
            property: "presence"
            from: 0
            to: 1
            duration: 420
            easing.type: Easing.OutQuad
        }

        NumberAnimation {
            target: eye
            property: "approach"
            from: 1
            to: 0
            duration: 1150
            easing.type: Easing.OutCubic
        }

        NumberAnimation {
            target: stage
            property: "spin"
            from: -1.7
            to: 0
            duration: 1500
            easing.type: Easing.OutQuart
        }

        SequentialAnimation {
            PropertyAction {
                target: stage
                property: "unfurl"
                value: 0
            }

            PauseAnimation {
                duration: 420
            }

            NumberAnimation {
                target: stage
                property: "unfurl"
                from: 0
                to: 1
                duration: 800
                easing.type: Easing.OutCubic
            }

        }

    }

    // The nucleus's bloom: it waits, rises from wherever it stands, then
    // fades. bloomAt() sets it going.
    SequentialAnimation {
        id: blooming

        PauseAnimation {
            id: bloomDelay
        }

        NumberAnimation {
            id: bloomRise

            target: stage
            property: "bloom"
            easing.type: Easing.OutSine
        }

        NumberAnimation {
            id: bloomFall

            target: stage
            property: "bloom"
            to: 0
            easing.type: Easing.InOutSine
        }

    }

    NumberAnimation {
        id: rushAnimation

        target: stage
        property: "rush"
        from: 0
        to: 1
        duration: 380
        easing.type: Easing.Linear
        onFinished: stage.rushed()
    }

    SequentialAnimation on drift {
        loops: Animation.Infinite

        NumberAnimation {
            from: -1
            to: 1
            duration: 4400
            easing.type: Easing.InOutSine
        }

        NumberAnimation {
            from: 1
            to: -1
            duration: 4400
            easing.type: Easing.InOutSine
        }

    }

    SequentialAnimation on breath {
        loops: Animation.Infinite

        NumberAnimation {
            from: 0
            to: 1
            duration: 2600
            easing.type: Easing.InOutSine
        }

        NumberAnimation {
            from: 1
            to: 0
            duration: 2600
            easing.type: Easing.InOutSine
        }

    }

}
