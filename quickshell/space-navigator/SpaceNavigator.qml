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
    // The window the first m picked up, and where the navigator stood when
    // it did, so a cancel can put the picture back. Hyprland does not change
    // until the second m.
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
    // The focus or the move has been started in its own session. The host
    // closes; the command runs after the host layer is gone.
    signal finished()
    // Escape or q with no window carried.
    signal closeRequested()

    // open reads the space; the picture opens once it is read.
    function open() {
        lister.running = true;
    }

    // leave drops a carried window, which Hyprland never saw, so the host
    // can stop showing the navigator. It refuses while a focus or a move is
    // under way.
    function leave() {
        if (navigator.confirming)
            return false;

        if (navigator.carrying)
            navigator.cancelCarry();

        return true;
    }

    // cancel drops a carried window, or with no window carried asks the host
    // to close.
    function cancel() {
        if (navigator.confirming)
            return ;

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
            navigator.column = Math.max(0, workspaces.findIndex((workspace) => {
                return workspace.id === navigator.activeId;
            }));
            navigator.columnSteps = navigator.column;
            navigator.chosenLayer = 0;
            navigator.layerSteps = 0;
            navigator.workspaces = workspaces;
        } catch (error) {
            console.warn("Unable to parse tesseract space:", error);
            navigator.workspaces = [];
        }
        navigator.loading = false;
        stage.arrive();
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

    // carry picks up the selected window on the first m and puts it down on
    // the second. With no window selected, m does nothing.
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
    // host's layer is gone, then focuses or moves. Hyprland refuses a
    // window focus while that exclusive layer is mapped.
    function commandAfterUnmap(action, workspace, address) {
        const script = action === "move-focus" ? "export TESSERACT_SPACE_WAIT_LAYER=1; \"$0\" space move \"$1\" \"$2\" && exec \"$0\" space focus \"$1\" \"$2\"" : action === "move" ? "export TESSERACT_SPACE_WAIT_LAYER=1; exec \"$0\" space move \"$1\" \"$2\"" : "export TESSERACT_SPACE_WAIT_LAYER=1; exec \"$0\" space focus \"$1\" \"$2\"";
        return ["setsid", "-f", "bash", "-c", script, navigator.command, workspace, address];
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

    // cancelCarry puts the picture back where it stood before the first m,
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

    function handleKey(event) {
        event.accepted = true;
        if (navigator.confirming)
            return ;

        if (navigator.isEscapeKey(event) || event.key === Qt.Key_Q)
            navigator.cancel();
        else if (navigator.isReturnKey(event))
            navigator.confirm();
        else if (event.key === Qt.Key_M)
            navigator.carry();
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

    // setsid -f returns as soon as the command is in its own session. The
    // host closes on finished(); the command focuses or moves after the
    // layer is gone.
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
