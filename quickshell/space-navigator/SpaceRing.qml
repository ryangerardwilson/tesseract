import QtQuick
import QtQuick.Shapes

// SpaceRing is a thin ring, or half of one, already projected to stage
// points: a hairline over a faint wide glow.
Shape {
    id: ring

    required property SpaceCamera lens
    property var points: []
    property real light: 1
    property real weight: 1

    anchors.fill: parent
    preferredRendererType: Shape.CurveRenderer
    visible: light > 0.004 && points.length > 1

    ShapePath {
        strokeColor: Qt.rgba(1, 1, 1, 0.05 * ring.light)
        strokeWidth: 5 * ring.weight * ring.lens.unit
        fillColor: "transparent"
        capStyle: ShapePath.FlatCap
        joinStyle: ShapePath.RoundJoin

        PathPolyline {
            path: ring.points
        }

    }

    ShapePath {
        strokeColor: Qt.rgba(1, 1, 1, Math.min(1, 0.55 * ring.light))
        strokeWidth: ring.weight * ring.lens.unit
        fillColor: "transparent"
        capStyle: ShapePath.FlatCap
        joinStyle: ShapePath.RoundJoin

        PathPolyline {
            path: ring.points
        }

    }

}
