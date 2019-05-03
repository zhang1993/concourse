module Views.Styles exposing
    ( breadcrumbComponent
    , breadcrumbContainer
    , breadcrumbItem
    , concourseLogo
    , pageBelowTopBar
    , pageHeaderHeight
    , pageIncludingTopBar
    , pauseToggleIcon
    , pipelinePageIncludingTopBar
    , pipelineTopBar
    , topBar
    )

import Colors
import Html
import Html.Attributes exposing (style)
import Routes
import Views.Views as Views


pageHeaderHeight : Float
pageHeaderHeight =
    54


pageIncludingTopBar : List (Html.Attribute msg)
pageIncludingTopBar =
    [ style "-webkit-font-smoothing" "antialiased"
    , style "font-weight" "700"
    , style "height" "100%"
    ]


pipelinePageIncludingTopBar : List Views.Style
pipelinePageIncludingTopBar =
    [ Views.style "-webkit-font-smoothing" "antialiased"
    , Views.style "font-weight" "700"
    , Views.style "height" "100%"
    ]


pageBelowTopBar : Routes.Route -> List (Html.Attribute msg)
pageBelowTopBar route =
    [ style "padding-top" "54px"
    , style "height" "100%"
    ]
        ++ (case route of
                Routes.Pipeline _ ->
                    [ style "box-sizing" "border-box" ]

                Routes.Dashboard _ ->
                    [ style "box-sizing" "border-box"
                    , style "display" "flex"
                    , style "padding-bottom" "50px"
                    ]

                _ ->
                    []
           )


topBar : Bool -> List Views.Style
topBar isPaused =
    [ Views.style "position" "fixed"
    , Views.style "top" "0"
    , Views.style "width" "100%"
    , Views.style "z-index" "999"
    , Views.style "display" "flex"
    , Views.style "justify-content" "space-between"
    , Views.style "font-weight" "700"
    , Views.style "background-color" <|
        if isPaused then
            Colors.paused

        else
            Colors.frame
    ]


pipelineTopBar : Bool -> List Views.Style
pipelineTopBar isPaused =
    [ Views.style "position" "fixed"
    , Views.style "top" "0"
    , Views.style "width" "100%"
    , Views.style "z-index" "999"
    , Views.style "display" "flex"
    , Views.style "justify-content" "space-between"
    , Views.style "font-weight" "700"
    , Views.style "background-color" <|
        if isPaused then
            Colors.paused

        else
            Colors.frame
    ]


concourseLogo : List Views.Style
concourseLogo =
    [ Views.style "background-image" "url(/public/images/concourse-logo-white.svg)"
    , Views.style "background-position" "50% 50%"
    , Views.style "background-repeat" "no-repeat"
    , Views.style "background-size" "42px 42px"
    , Views.style "width" "54px"
    , Views.style "height" "54px"
    ]


breadcrumbContainer : List Views.Style
breadcrumbContainer =
    [ Views.style "flex-grow" "1" ]


breadcrumbComponent : String -> List Views.Style
breadcrumbComponent componentType =
    [ Views.style "background-image" <|
        "url(/public/images/ic-breadcrumb-"
            ++ componentType
            ++ ".svg)"
    , Views.style "background-repeat" "no-repeat"
    , Views.style "background-size" "contain"
    , Views.style "display" "inline-block"
    , Views.style "vertical-align" "middle"
    , Views.style "height" "16px"
    , Views.style "width" "32px"
    , Views.style "margin-right" "10px"
    ]


breadcrumbItem : Bool -> List Views.Style
breadcrumbItem clickable =
    [ Views.style "display" "flex"
    , Views.style "font-size" "18px"
    , Views.style "padding" "0 10px"
    , Views.style "line-height" "54px"
    , Views.style "cursor" <|
        if clickable then
            "pointer"

        else
            "default"
    ]


pauseToggleIcon :
    { isHovered : Bool
    , isClickable : Bool
    , margin : String
    }
    -> List (Html.Attribute msg)
pauseToggleIcon { isHovered, isClickable, margin } =
    [ style "margin" margin
    , style "opacity" <|
        if isHovered then
            "1"

        else
            "0.5"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    ]
