module Views.Views exposing
    ( Identifier(..)
    , NodeType(..)
    , Style
    , View
    , a
    , childAt
    , dd
    , div
    , dl
    , dt
    , find
    , getStyle
    , h3
    , hasAttribute
    , img
    , li
    , logLine
    , nodeType
    , p
    , pre
    , span
    , style
    , svg
    , table
    , td
    , text
    , toHtml
    , tr
    , ul
    )

import Ansi.Log
import Html
import Html.Attributes
import List.Extra
import Svg


type View msg
    = View Identifier NodeType (List Style) (List (Html.Attribute msg)) (List (View msg))


type NodeType
    = Div
    | A
    | Ul
    | Dl
    | Dt
    | Dd
    | Li
    | Table
    | Tr
    | Td
    | H3
    | Svg
    | P
    | Img
    | Span
    | Pre
    | LogLine Ansi.Log.Line
    | Text String


type Identifier
    = Id String
    | Unidentified


type Style
    = Style String String


div : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
div id styles children =
    View id Div styles children


ul : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
ul id styles children =
    View id Ul styles children


li : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
li id styles children =
    View id Li styles children


table : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
table id styles children =
    View id Table styles children


tr : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
tr id styles children =
    View id Tr styles children


td : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
td id styles children =
    View id Td styles children


a : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
a id styles children =
    View id A styles children


h3 : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
h3 id styles children =
    View id H3 styles children


svg : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
svg id styles children =
    View id Svg styles children


img : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
img id styles children =
    View id Img styles children


p : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
p id styles children =
    View id P styles children


dl : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
dl id styles children =
    View id Dl styles children


dt : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
dt id styles children =
    View id Dt styles children


dd : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
dd id styles children =
    View id Dd styles children


span : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
span id styles children =
    View id Span styles children


pre : Identifier -> List Style -> List (Html.Attribute msg) -> List (View msg) -> View msg
pre id styles children =
    View id Pre styles children


logLine : Ansi.Log.Line -> View msg
logLine line =
    View Unidentified (LogLine line) [] [] []


text : String -> View msg
text textContent =
    View Unidentified (Text textContent) [] [] []


getStyle : String -> View msg -> String
getStyle property (View _ _ styles _ _) =
    List.Extra.find (isProperty property) styles
        |> Maybe.map value
        |> Maybe.withDefault ""


style : String -> String -> Style
style =
    Style


toHtml : View msg -> Html.Html msg
toHtml (View identifier nt styles attributes children) =
    let
        idAttrs =
            case identifier of
                Id id ->
                    [ Html.Attributes.id id ]

                Unidentified ->
                    []
    in
    case nt of
        Div ->
            Html.div
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        A ->
            Html.a
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Ul ->
            Html.ul
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Li ->
            Html.li
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Table ->
            Html.table
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Tr ->
            Html.tr
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Td ->
            Html.td
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        H3 ->
            Html.h3
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Svg ->
            Svg.svg
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Img ->
            Html.img
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        P ->
            Html.p
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Dl ->
            Html.dl
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Dt ->
            Html.dt
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Dd ->
            Html.dd
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Span ->
            Html.span
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        Pre ->
            Html.pre
                (idAttrs ++ List.map toAttr styles ++ attributes)
                (List.map toHtml children)

        LogLine line ->
            Ansi.Log.viewLine line

        Text string ->
            Html.text string


toAttr : Style -> Html.Attribute msg
toAttr (Style prop v) =
    Html.Attributes.style prop v


isProperty : String -> Style -> Bool
isProperty property (Style prop _) =
    property == prop


value : Style -> String
value (Style _ v) =
    v


find : String -> View msg -> View msg
find id =
    find_ id >> Maybe.withDefault (text "")


find_ : String -> View msg -> Maybe (View msg)
find_ id view =
    case view of
        View identifier _ _ _ children ->
            if identifier == Id id then
                Just view

            else
                children |> List.filterMap (find_ id) |> List.head


nodeType : View msg -> NodeType
nodeType (View _ nt _ _ _) =
    nt


hasAttribute : Html.Attribute msg -> View msg -> Bool
hasAttribute attribute (View _ _ _ attributes _) =
    List.member attribute attributes


childAt : Int -> View msg -> View msg
childAt n (View _ _ _ _ children) =
    children |> List.Extra.getAt n |> Maybe.withDefault (text "")
