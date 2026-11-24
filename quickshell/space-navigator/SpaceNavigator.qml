import QtQuick
import Quickshell.Io

// SpaceNavigator owns the navigator: the space it reads through `space
// list`, the selection and the carried window, the keys, and the focus and
// move commands. SpaceStage only draws it. A shell hosts it: the `tesseract
// open space` overlay fills its panel with it, and another Quickshell shell
// can stand it in part of its own surface. The host names the binary, calls
// open() when the navigator shows, passes it the keys, unmaps its window on
// hideRequested(), and closes on finished() and closeRequested().
Item {
    id: navigator

    // The tesseract binary whose space commands list, focus, and move.
    property string command: "tesseract"
    // How dark the void behind the system is. A host with its own backdrop
    // clears it.
    property alias voidOpacity: stage.voidOpacity
    property var workspaces: []
    property int activeId: 0
    property int column: 0
    property int chosenLayer: 0
    // The moves on each axis counted without wrapping, so the picture turns
    // the short way round when a selection cycles past an end.
    property int columnSteps: 0
    property int layerSteps: 0
    property bool loading: true
    property bool confirming: false
    // A place the host brought back from an earlier showing: the workspace id
    // and the lit app's address. applyList opens on it; workspace 0 is none.
    property int restoreWorkspace: 0
    property string restoreAddress: ""
    // The window the first s picked up, and where the navigator stood when
    // it did, so a cancel can put the picture back. Hyprland does not change
    // until the second s.
    property var carried: null
    property var carriedFrom: null
    readonly property bool carrying: carried !== null
    // The space as the picture shows it: with a window carried, as if it
    // were put down at the selected workspace and layer.
    readonly property var shownWorkspaces: carrying ? carriedSpace(workspaces, carried, column, chosenLayer) : workspaces
    readonly property var selectedWorkspace: shownWorkspaces.length > 0 ? shownWorkspaces[column] : null
    readonly property var layers: selectedWorkspace ? (selectedWorkspace.layers || []) : []
    readonly property var selectedLayer: chosenLayer < layers.length ? layers[chosenLayer] : null

    // The host unmaps its window: Hyprland refuses window focus while an
    // exclusive layer surface is mapped.
    signal hideRequested()
    // The focus, the move, the new workspace, or the close has been started
    // in its own session. The host closes; the command runs after the host
    // layer is gone.
    signal finished()
    // Escape or q with no window carried.
    signal closeRequested()

    // open reads the space; the picture opens once it is read.
    function open() {
        lister.running = true;
    }

    // place is where the selection rests: the workspace id and the lit app's
    // address, "" on a workspace with no apps, and workspace 0 before the
    // space is read. A host reads it after leave(), so a carried app is
    // already back where it was picked up.
    function place() {
        const workspace = navigator.selectedWorkspace;
        return {
            "workspace": workspace ? workspace.id : 0,
            "address": navigator.selectedLayer ? String(navigator.selectedLayer.address) : ""
        };
    }

    // leave drops a carried window, which Hyprland never saw, so the host
    // can stop showing the navigator. It refuses while a focus or a move is
    // under way.
    function leave() {
        if (navigator.confirming)
            return false;

        closeArm.stop();
        if (navigator.carrying)
            navigator.cancelCarry();

        return true;
    }

    // cancel drops a carried window, or with no window carried asks the host
    // to close.
    function cancel() {
        if (navigator.confirming)
            return ;

        closeArm.stop();
        if (navigator.carrying)
            navigator.cancelCarry();
        else
            navigator.closeRequested();
    }

    function applyList(text) {
        try {
            const parsed = JSON.parse(text);
            const workspaces = parsed.workspaces || [];
            navigator.activeId = parsed.active || 0;
            // Both step counts start at the place, so the orbit settles onto
            // it instead of turning in from another workspace.
            const start = navigator.startPlace(workspaces);
            navigator.column = start.column;
            navigator.columnSteps = start.column;
            navigator.chosenLayer = start.layer;
            navigator.layerSteps = start.layer;
            navigator.workspaces = workspaces;
        } catch (error) {
            console.warn("Unable to parse tesseract space:", error);
            navigator.workspaces = [];
        }
        navigator.loading = false;
        stage.arrive();
    }

    // startPlace is where the picture opens: the place the host brought
    // back, or else the active workspace's front app. A workspace that is
    // gone gives way to the next lower one, or else the next higher, and an
    // app that is not on the workspace that opens gives way to its front app.
    function startPlace(workspaces) {
        let column = workspaces.findIndex((workspace) => {
            return workspace.id === navigator.activeId;
        });
        const restoring = navigator.restoreWorkspace > 0 && workspaces.length > 0;
        if (restoring)
            column = navigator.nearestColumn(workspaces, navigator.restoreWorkspace);

        column = Math.max(0, column);
        const apps = workspaces.length > 0 ? (workspaces[column].layers || []) : [];
        const layer = restoring && navigator.restoreAddress !== "" ? apps.findIndex((app) => {
            return app.address === navigator.restoreAddress;
        }) : 0;
        return {
            "column": column,
            "layer": Math.max(0, layer)
        };
    }

    // nearestColumn is the column of workspace id, or else of the next lower
    // id, or else of the next higher. The X axis is in ascending id order.
    function nearestColumn(workspaces, id) {
        let column = 0;
        for (let index = 0; index < workspaces.length && workspaces[index].id <= id; index++)
            column = index;
        return column;
    }

    // moveColumn steps on X, cycles past either end, and starts the new
    // workspace at its front layer.
    function moveColumn(delta) {
        const count = navigator.workspaces.length;
        if (count < 2)
            return ;

        navigator.columnSteps += delta;
        navigator.column = (navigator.column + delta + count) % count;
        navigator.layerSteps = 0;
        navigator.chosenLayer = 0;
    }

    // moveLayer steps on Z, -1 toward the front and +1 deeper, cycling
    // between the front layer and the deepest.
    function moveLayer(delta) {
        const count = navigator.layers.length;
        if (count < 2)
            return ;

        navigator.layerSteps += delta;
        navigator.chosenLayer = (navigator.chosenLayer + delta + count) % count;
    }

    // carriedSpace is the space with the carried window off its old
    // workspace and put down at `column` and `layer`, the other layers in
    // their order.
    function carriedSpace(workspaces, carried, column, layer) {
        return workspaces.map((workspace, index) => {
            const layers = (workspace.layers || []).filter((each) => {
                return each.address !== carried.address;
            });
            if (index === column)
                layers.splice(Math.min(layer, layers.length), 0, carried);

            return Object.assign({
            }, workspace, {
                "layers": layers
            });
        });
    }

    // shortWay is `delta` steps round a cycle of `count`, taken the short
    // way.
    function shortWay(delta, count) {
        if (count < 1)
            return 0;

        const way = ((delta % count) + count) % count;
        return way > count / 2 ? way - count : way;
    }

    // carry picks up the selected window on the first s and puts it down on
    // the second. With no window selected, s does nothing.
    function carry() {
        if (navigator.carrying) {
            navigator.putDown();
            return ;
        }
        if (!navigator.selectedLayer)
            return ;

        navigator.carriedFrom = {
            "column": navigator.column,
            "layer": navigator.chosenLayer
        };
        navigator.carried = navigator.selectedLayer;
    }

    // commandAfterUnmap starts tesseract in its own session. setsid -f
    // returns at once so the host can close. The command waits until this
    // host's layer is gone, then focuses, moves, adds a workspace, or closes
    // one. Hyprland refuses a window focus while that exclusive layer is
    // mapped, and a save dialog needs the keyboard. In each script "$0" is
    // the binary, "$1" the workspace, and "$2" the window.
    function commandAfterUnmap(action, workspace, address) {
        const scripts = {
            "focus": "exec \"$0\" space focus \"$1\" \"$2\"",
            "move": "exec \"$0\" space move \"$1\" \"$2\"",
            "move-focus": "\"$0\" space move \"$1\" \"$2\" && exec \"$0\" space focus \"$1\" \"$2\"",
            "new": "exec \"$0\" space new",
            "close": "exec \"$0\" space close \"$1\""
        };
        const script = "export TESSERACT_SPACE_WAIT_LAYER=1; " + scripts[action];
        return ["setsid", "-f", "bash", "-c", script, navigator.command, workspace || "", address || ""];
    }

    // startAfterUnmap asks the host to hide and starts command; finished()
    // follows once the command is in its own session.
    function startAfterUnmap(command) {
        focuser.command = command;
        navigator.hideRequested();
        focuser.running = true;
    }

    // select puts the selection on workspace index, at its front app.
    function select(index) {
        navigator.columnSteps += navigator.shortWay(index - navigator.column, navigator.workspaces.length);
        navigator.column = index;
        navigator.chosenLayer = 0;
        navigator.layerSteps = 0;
    }

    // addWorkspace adds the workspace one past the highest id and selects
    // it. The host hides and closes, and the operator lands on the new
    // workspace once the host layer is gone. No window moves.
    function addWorkspace() {
        if (navigator.carrying || navigator.loading)
            return ;

        const id = 1 + navigator.workspaces.reduce((highest, workspace) => {
            return Math.max(highest, workspace.id);
        }, 0);
        navigator.confirming = true;
        navigator.workspaces = navigator.workspaces.concat([{
            "id": id,
            "name": String(id),
            "layers": []
        }]);
        navigator.select(navigator.workspaces.length - 1);
        navigator.startAfterUnmap(navigator.commandAfterUnmap("new"));
    }

    // pressClose arms dd on a first d and closes the selected workspace on a
    // second d within closeArm's 400 ms. A d that only repeats while held,
    // a carried window, or one workspace on the orbit closes nothing.
    function pressClose(repeated) {
        if (repeated || navigator.carrying || navigator.workspaces.length < 2 || !navigator.selectedWorkspace)
            return ;

        if (!closeArm.running) {
            closeArm.restart();
            return ;
        }
        closeArm.stop();
        navigator.closeWorkspace();
    }

    // closeWorkspace takes the selected workspace off the orbit and selects
    // the next lower one, or else the next higher. Each app on it gets one
    // graceful close once the host layer is gone, so a save dialog can take
    // the keyboard.
    function closeWorkspace() {
        const closed = navigator.selectedWorkspace.id;
        const index = navigator.column;
        navigator.confirming = true;
        navigator.workspaces = navigator.workspaces.filter((workspace) => {
            return workspace.id !== closed;
        });
        navigator.select(Math.max(0, index - 1));
        navigator.startAfterUnmap(navigator.commandAfterUnmap("close", String(closed)));
    }

    // putDown leaves the window where h, j, k, and l carried it. Put down in
    // a new place, the host closes and the window moves through `tesseract
    // space move` once the layer is gone; put down where it was picked up,
    // nothing changes.
    function putDown() {
        const from = navigator.carriedFrom;
        if (navigator.column === from.column && navigator.chosenLayer === from.layer) {
            navigator.carried = null;
            return ;
        }
        const workspace = String(navigator.selectedWorkspace.id);
        const address = String(navigator.carried.address);
        navigator.confirming = true;
        // At the front of another workspace, the move is the whole commit.
        // Behind another window, or back on its own workspace, no z-order
        // dispatcher has documented arguments, so focus lands it on top.
        const action = navigator.chosenLayer > 0 || navigator.column === from.column ? "move-focus" : "move";
        focuser.command = navigator.commandAfterUnmap(action, workspace, address);
        navigator.hideRequested();
        focuser.running = true;
    }

    // cancelCarry puts the picture back where it stood before the first s,
    // the short way round, and dispatches nothing.
    function cancelCarry() {
        const from = navigator.carriedFrom;
        if (from.column === navigator.column) {
            navigator.layerSteps += navigator.shortWay(from.layer - navigator.chosenLayer, navigator.layers.length);
        } else {
            navigator.columnSteps += navigator.shortWay(from.column - navigator.column, navigator.workspaces.length);
            navigator.column = from.column;
            navigator.layerSteps = from.layer;
        }
        navigator.chosenLayer = from.layer;
        navigator.carried = null;
    }

    function confirm() {
        if (navigator.confirming || navigator.carrying || !navigator.selectedWorkspace || !navigator.selectedLayer)
            return ;

        navigator.confirming = true;
        focuser.command = navigator.commandAfterUnmap("focus", String(navigator.selectedWorkspace.id), String(navigator.selectedLayer.address));
        // The chosen electron rushes out of its shell toward the eye first;
        // focusChosen runs when it arrives.
        stage.rushIn();
    }

    function focusChosen() {
        // Hyprland refuses window focus while an exclusive layer surface is
        // mapped, so the host closes and the detached command focuses once
        // that layer is gone.
        navigator.hideRequested();
        focuser.running = true;
    }

    function isEscapeKey(event) {
        return event.key === Qt.Key_Escape || (event.key === Qt.Key_BracketLeft && (event.modifiers & Qt.ControlModifier));
    }

    // Enter arrives as Ctrl+M, the same as in the session radial.
    function isReturnKey(event) {
        return event.key === Qt.Key_Return || event.key === Qt.Key_Enter || (event.key === Qt.Key_M && (event.modifiers & Qt.ControlModifier));
    }

    // A key with no Ctrl, Alt, Shift, or Super held.
    function isBareKey(event, key) {
        const held = Qt.ControlModifier | Qt.AltModifier | Qt.ShiftModifier | Qt.MetaModifier;
        return event.key === key && (event.modifiers & held) === 0;
    }

    // handleKey takes the navigator's keys. Ctrl+M is Enter and is checked
    // before s, so it never shifts. Bare n adds a workspace and bare dd
    // closes the selected one; any other key forgets a waiting d first and
    // then does its own job. Bare m is not a navigator key: it stays
    // unaccepted, so a host can bind it.
    function handleKey(event) {
        event.accepted = true;
        if (navigator.confirming)
            return ;

        const closing = navigator.isBareKey(event, Qt.Key_D);
        if (!closing)
            closeArm.stop();

        if (navigator.isEscapeKey(event) || event.key === Qt.Key_Q)
            navigator.cancel();
        else if (navigator.isReturnKey(event))
            navigator.confirm();
        else if (event.key === Qt.Key_S && !(event.modifiers & Qt.ControlModifier))
            navigator.carry();
        else if (navigator.isBareKey(event, Qt.Key_N))
            navigator.addWorkspace();
        else if (closing)
            navigator.pressClose(event.isAutoRepeat);
        else if (event.key === Qt.Key_Left || event.key === Qt.Key_H)
            navigator.moveColumn(-1);
        else if (event.key === Qt.Key_Right || event.key === Qt.Key_L)
            navigator.moveColumn(1);
        else if (event.key === Qt.Key_Up || event.key === Qt.Key_K)
            navigator.moveLayer(-1);
        else if (event.key === Qt.Key_Down || event.key === Qt.Key_J)
            navigator.moveLayer(1);
        else
            event.accepted = false;
    }

    Process {
        id: lister

        running: false
        command: [navigator.command, "space", "list"]
        onRunningChanged: {
            if (!running)
                navigator.loading = false;

        }

        stdout: StdioCollector {
            onStreamFinished: navigator.applyList(this.text)
        }

    }

    // The first d of dd waits here for its second.
    Timer {
        id: closeArm

        interval: 400
    }

    // setsid -f returns as soon as the command is in its own session. The
    // host closes on finished(); the command focuses, moves, adds a
    // workspace, or closes one after the layer is gone.
    Process {
        id: focuser

        running: false
        onRunningChanged: {
            if (!running)
                navigator.finished();

        }
    }

    SpaceStage {
        id: stage

        anchors.fill: parent
        workspaces: navigator.shownWorkspaces
        activeId: navigator.activeId
        column: navigator.column
        chosenLayer: navigator.chosenLayer
        columnSteps: navigator.columnSteps
        layerSteps: navigator.layerSteps
        loading: navigator.loading
        carrying: navigator.carrying
        onRushed: navigator.focusChosen()
    }

}
