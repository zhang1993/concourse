module Views.Views exposing
    ( Identifier(..)
    , NodeType(..)
    , Style
    , View
    , a
    , childAt
    , div
    , find
    , getStyle
    , hasAttribute
    , li
    , nodeType
    , style
    , table
    , td
    , text
    , toHtml
    , tr
    , ul
    )

import Html
import Html.Attributes
import List.Extra


type View msg
    = View Identifier NodeType (List Style) (List (Html.Attribute msg)) (List (View msg))


type NodeType
    = Div
    | A
    | Ul
    | Li
    | Table
    | Tr
    | Td
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

        Text string ->
            Html.text string


toAttr : Style -> Html.Attribute msg
toAttr (Style p v) =
    Html.Attributes.style p v


isProperty : String -> Style -> Bool
isProperty property (Style p _) =
    property == p


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
