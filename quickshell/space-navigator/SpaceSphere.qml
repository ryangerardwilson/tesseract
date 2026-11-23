import QtQuick
import QtQuick.Shapes

// SpaceSphere is a glass sphere centred on its own position: a dark body that
// hides what passes behind it, a bright rim, a soft highlight, a light inside
// as `glow` rises, and a bloom around it. It is drawn at a fixed size and
// scaled, and the curve renderer keeps every edge sharp at any size. Its
// light is white unless it is given a warmer core or a cooler rim.
Item {
    id: sphere

    property real radius: 40
    property real light: 1
    property real glow: 0
    // The tint of the light inside, and of the rim and the bloom around it.
    property color core: "white"
    property color rim: "white"
    // Children are laid out on the sphere's 200 by 200 face and scale with it.
    default property alias content: face.data

    Item {
        id: face

        x: -100
        y: -100
        width: 200
        height: 200
        scale: sphere.radius / 100
        visible: sphere.radius > 0.3

        Shape {
            anchors.fill: parent
            preferredRendererType: Shape.CurveRenderer

            ShapePath {
                strokeColor: "transparent"

                fillGradient: RadialGradient {
                    centerX: 100
                    centerY: 100
                    centerRadius: 175
                    focalX: 100
                    focalY: 100

                    GradientStop {
                        position: 0.55
                        color: Qt.rgba(sphere.rim.r, sphere.rim.g, sphere.rim.b, (0.05 + 0.11 * sphere.glow) * sphere.light)
                    }

                    GradientStop {
                        position: 0.7
                        color: Qt.rgba(sphere.rim.r, sphere.rim.g, sphere.rim.b, (0.018 + 0.04 * sphere.glow) * sphere.light)
                    }

                    GradientStop {
                        position: 1
                        color: Qt.rgba(sphere.rim.r, sphere.rim.g, sphere.rim.b, 0)
                    }

                }

                PathAngleArc {
                    centerX: 100
                    centerY: 100
                    radiusX: 175
                    radiusY: 175
                    startAngle: 0
                    sweepAngle: 360
                }

            }

            ShapePath {
                strokeColor: "transparent"
                fillColor: Qt.rgba(0.012, 0.012, 0.016, 0.9)

                PathAngleArc {
                    centerX: 100
                    centerY: 100
                    radiusX: 100
                    radiusY: 100
                    startAngle: 0
                    sweepAngle: 360
                }

            }

            ShapePath {
                strokeColor: "transparent"

                fillGradient: RadialGradient {
                    centerX: 100
                    centerY: 108
                    centerRadius: 100
                    focalX: 100
                    focalY: 108

                    GradientStop {
                        position: 0
                        color: Qt.rgba(sphere.core.r, sphere.core.g, sphere.core.b, 0.5 * sphere.glow)
                    }

                    GradientStop {
                        position: 0.45
                        color: Qt.rgba(sphere.core.r, sphere.core.g, sphere.core.b, 0.2 * sphere.glow)
                    }

                    GradientStop {
                        position: 0.85
                        color: Qt.rgba(sphere.core.r, sphere.core.g, sphere.core.b, 0.05 * sphere.glow)
                    }

                    GradientStop {
                        position: 1
                        color: Qt.rgba(sphere.core.r, sphere.core.g, sphere.core.b, 0)
                    }

                }

                PathAngleArc {
                    centerX: 100
                    centerY: 100
                    radiusX: 100
                    radiusY: 100
                    startAngle: 0
                    sweepAngle: 360
                }

            }

            ShapePath {
                strokeColor: Qt.rgba(sphere.rim.r, sphere.rim.g, sphere.rim.b, Math.min(1, (0.4 + 0.35 * sphere.glow) * sphere.light))
                strokeWidth: Math.max(1.2, 1.1 / Math.max(0.01, face.scale))

                fillGradient: RadialGradient {
                    centerX: 100
                    centerY: 100
                    centerRadius: 100
                    focalX: 60
                    focalY: 52
                    focalRadius: 0

                    GradientStop {
                        position: 0
                        color: Qt.rgba(1, 1, 1, 0.3 * sphere.light)
                    }

                    GradientStop {
                        position: 0.28
                        color: Qt.rgba(1, 1, 1, 0.09 * sphere.light)
                    }

                    GradientStop {
                        position: 0.7
                        color: Qt.rgba(1, 1, 1, 0.03 * sphere.light)
                    }

                    GradientStop {
                        position: 0.9
                        color: Qt.rgba(sphere.rim.r, sphere.rim.g, sphere.rim.b, 0.1 * sphere.light)
                    }

                    GradientStop {
                        position: 1
                        color: Qt.rgba(sphere.rim.r, sphere.rim.g, sphere.rim.b, 0.38 * sphere.light)
                    }

                }

                PathAngleArc {
                    centerX: 100
                    centerY: 100
                    radiusX: 100
                    radiusY: 100
                    startAngle: 0
                    sweepAngle: 360
                }

            }

        }

        Shape {
            x: 28
            y: 26
            width: 70
            height: 40
            rotation: -36
            preferredRendererType: Shape.CurveRenderer

            ShapePath {
                strokeColor: "transparent"

                fillGradient: RadialGradient {
                    centerX: 35
                    centerY: 20
                    centerRadius: 35
                    focalX: 35
                    focalY: 20

                    GradientStop {
                        position: 0
                        color: Qt.rgba(1, 1, 1, 0.5 * sphere.light)
                    }

                    GradientStop {
                        position: 1
                        color: Qt.rgba(1, 1, 1, 0)
                    }

                }

                PathAngleArc {
                    centerX: 35
                    centerY: 20
                    radiusX: 35
                    radiusY: 20
                    startAngle: 0
                    sweepAngle: 360
                }

            }

        }

    }

}
